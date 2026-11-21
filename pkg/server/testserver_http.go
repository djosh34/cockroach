// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/security/certconfig"
	"github.com/cockroachdb/cockroach/pkg/security/securityassets"
	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/cockroach/pkg/server/apiconstants"
	"github.com/cockroachdb/cockroach/pkg/server/authserver"
	"github.com/cockroachdb/cockroach/pkg/sql/sessiondata"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/util"
	"github.com/cockroachdb/errors"
)

// httpTestServer is embedded in testServer / TenantServer to
// provide the HTTP API subset of ApplicationLayerInterface.
type httpTestServer struct {
	t struct {
		// We need a sub-struct to avoid ambiguous overlap with the fields
		// of *Server, which are also embedded in testServer.
		authentication authserver.Server
		sqlServer      *SQLServer
		tenantName     roachpb.TenantName
		admin          *adminServer
		status         *statusServer
	}

	// authClient holds the clients of GetAuthenticatedHTTPClient, the
	// non-admin one first.
	authClient [2]struct {
		httpClient http.Client
		once       sync.Once
		err        error
	}
}

type tenantHeaderDecorator struct {
	http.RoundTripper

	tenantName roachpb.TenantName
}

func (t tenantHeaderDecorator) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.tenantName != "" {
		req.Header.Add(TenantSelectHeader, string(t.tenantName))
	}
	return t.RoundTripper.RoundTrip(req)
}

var _ http.RoundTripper = &tenantHeaderDecorator{}

// AdminURL implements TestServerInterface.
func (ts *httpTestServer) AdminURL() *serverutils.TestURL {
	u := ts.t.sqlServer.cfg.Config.AdminURL()
	if ts.t.tenantName != "" {
		q := u.Query()
		q.Add(ClusterNameParamInQueryURL, string(ts.t.tenantName))
		u.RawQuery = q.Encode()
	}
	return &serverutils.TestURL{URL: u}
}

// httpClient returns an HTTP client that verifies the server with the test
// node CA and, unless user is empty, presents the user's test client cert.
func (ts *httpTestServer) httpClient(user string) (http.Client, error) {
	client := http.Client{Timeout: 10 * time.Second}
	if util.RaceEnabled {
		client.Timeout = 30 * time.Second
	}
	cfg := ts.t.sqlServer.cfg.Config
	var transport http.Transport
	if !cfg.Insecure && !cfg.DisableTLSForHTTP {
		read := func(name string) ([]byte, error) {
			return securityassets.GetLoader().ReadFile(filepath.Join(certconfig.EmbeddedDir, name))
		}
		caPEM, err := securityassets.GetLoader().ReadFile(cfg.Certs.NodeCA)
		if err != nil {
			return client, err
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return client, errors.New("no certificates in the node CA file")
		}
		tlsCfg := &tls.Config{RootCAs: pool}
		if user != "" {
			certPEM, err := read("client." + user + ".crt")
			if err != nil {
				return client, err
			}
			keyPEM, err := read("client." + user + ".key")
			if err != nil {
				return client, err
			}
			pair, err := tls.X509KeyPair(certPEM, keyPEM)
			if err != nil {
				return client, err
			}
			tlsCfg.Certificates = []tls.Certificate{pair}
		}
		transport.TLSClientConfig = tlsCfg
	}
	client.Transport = &tenantHeaderDecorator{
		RoundTripper: &transport,
		tenantName:   ts.t.tenantName,
	}
	return client, nil
}

// GetUnauthenticatedHTTPClient implements TestServerInterface.
func (ts *httpTestServer) GetUnauthenticatedHTTPClient() (http.Client, error) {
	return ts.httpClient("")
}

// GetAdminHTTPClient implements the TestServerInterface.
func (ts *httpTestServer) GetAdminHTTPClient() (http.Client, error) {
	return ts.GetAuthenticatedHTTPClient(true)
}

// GetAuthenticatedHTTPClient implements the TestServerInterface. The client
// presents the test client cert of apiconstants.TestingUser, or of
// apiconstants.TestingUserNoAdmin if isAdmin is false.
func (ts *httpTestServer) GetAuthenticatedHTTPClient(isAdmin bool) (http.Client, error) {
	authUser := apiconstants.TestingUserName()
	authIdx := 0
	if isAdmin {
		authIdx = 1
	} else {
		authUser = apiconstants.TestingUserNameNoAdmin()
	}
	authClient := &ts.authClient[authIdx]
	authClient.once.Do(func() {
		// The user needs to exist as the admin endpoints will check its role.
		if authClient.err = ts.CreateAuthUser(authUser, isAdmin); authClient.err != nil {
			return
		}
		authClient.httpClient, authClient.err = ts.httpClient(authUser.Normalized())
	})
	return authClient.httpClient, authClient.err
}

// CreateAuthUser is exported for use in tests.
func (ts *httpTestServer) CreateAuthUser(userName username.SQLUsername, isAdmin bool) error {
	if _, err := ts.t.sqlServer.internalExecutor.ExecEx(context.TODO(),
		"create-auth-user", nil,
		sessiondata.RootUserSessionDataOverride,
		fmt.Sprintf("CREATE USER %s", userName.Normalized()),
	); err != nil {
		return err
	}
	if isAdmin {
		if _, err := ts.t.sqlServer.internalExecutor.ExecEx(context.TODO(),
			"grant-admin", nil,
			sessiondata.RootUserSessionDataOverride,
			fmt.Sprintf("GRANT admin TO %s WITH ADMIN OPTION", userName.Normalized()),
		); err != nil {
			return err
		}
	}
	return nil
}
