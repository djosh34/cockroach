// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package authserver

import (
	"crypto/x509"
	"net/http"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/cockroach/pkg/sql"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/errors"
)

// ForwardedUserHeader carries the user of an HTTP request that one node
// forwards to another, for example to serve a request for a remote node's
// status. The receiving node trusts it only from a peer that presents a
// node certificate. An empty value means the request had no user.
const ForwardedUserHeader = "X-Patchroaddb-Forwarded-User"

// errNoUser is returned for a request without a user.
var errNoUser = errors.New("a client certificate is required")

type authenticationServer struct {
	cfg       *base.Config
	sqlServer SQLServerInterface
}

// UserFromRequest returns the user that an HTTP request authenticates as.
//
// A client certificate that chains to the client CA is the role of the
// first client rule it matches. A node certificate is root, unless the
// request carries ForwardedUserHeader: then it is the user named there,
// which the forwarding node has authenticated. The user must exist and be
// allowed to log in.
func (s *authenticationServer) UserFromRequest(r *http.Request) (username.SQLUsername, error) {
	if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
		return username.SQLUsername{}, errNoUser
	}
	execCfg := s.sqlServer.ExecutorConfig()
	cm, err := execCfg.RPCContext.GetCertificateManager()
	if err != nil {
		return username.SQLUsername{}, err
	}
	certs := r.TLS.PeerCertificates
	var user username.SQLUsername
	if cm.VerifyNode(certs, x509.ExtKeyUsageClientAuth) == nil {
		user = username.RootUserName()
		if v, forwarded := r.Header[http.CanonicalHeaderKey(ForwardedUserHeader)]; forwarded {
			if len(v) != 1 || v[0] == "" {
				return username.SQLUsername{}, errNoUser
			}
			user = username.MakeSQLUsernameFromPreNormalizedString(v[0])
		}
	} else {
		roles, err := cm.ClientRoles(certs)
		if err != nil {
			return username.SQLUsername{}, err
		}
		user = roles[0]
	}
	exists, _, canLogin, _, _, _, _, err := sql.GetUserSessionInitInfo(
		r.Context(), execCfg, user, "" /* databaseName */)
	if err != nil {
		return username.SQLUsername{}, err
	}
	if !exists {
		return username.SQLUsername{}, errors.Newf("user %s does not exist", user)
	}
	if !canLogin {
		return username.SQLUsername{}, errors.Newf("%s does not have login privilege", user)
	}
	return user, nil
}

// authenticationMux implements http.Handler. It authenticates each request
// by its client certificate before it calls its inner handler.
type authenticationMux struct {
	server *authenticationServer
	inner  http.Handler

	// allowAnonymous, if true, indicates that the authentication mux should
	// call its inner HTTP handler even if the request has no user. If there
	// is a user, the mux calls its inner handler with a context containing
	// the username.
	//
	// If allowAnonymous is false, the mux returns an error if there is no
	// user.
	allowAnonymous bool
}

func (am *authenticationMux) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	user, err := am.server.UserFromRequest(req)
	if err == nil {
		req = req.WithContext(ContextWithHTTPAuthInfo(req.Context(), user.Normalized(), 0))
	} else if !am.allowAnonymous {
		log.VInfof(req.Context(), 1, "HTTP authentication failed: %v", err)
		http.Error(w, "a valid client certificate is required", http.StatusUnauthorized)
		return
	}
	am.inner.ServeHTTP(w, req)
}
