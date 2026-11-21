// Copyright 2021 The Cockroach Authors.
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
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/rpc"
	"github.com/cockroachdb/cockroach/pkg/server/authserver"
	"github.com/cockroachdb/cockroach/pkg/util"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/syncutil"
	"github.com/cockroachdb/errors"
)

// RemoteNodeID is used to look up remote nodeID values to route
// requests to from a request's query params or a Cookie.
const RemoteNodeID = "remote_node_id"

// nodeProxy is an HTTP handler that inspects the request's cookie
// header and query params. If the `RemoteNodeID` key is found in
// either location, it will proxy the request to nodeID set in the
// value associated with the key. Otherwise, the request is handled
// by this node as usual.
type nodeProxy struct {
	scheme     string
	rpcContext *rpc.Context
	// authServer finds the user of a request before it is forwarded. It is
	// set when the server is made, before the HTTP listener starts.
	authServer           authserver.Server
	parseNodeID          ParseNodeIDFn
	getNodeIDHTTPAddress GetNodeIDHTTPAddressFn
	proxyCache           reverseProxyCache
}

// ParseNodeIDFn is a callback to parse strings into `roachpb.NodeID`
// instances. It exists to prevent a dependency on `gossip`.
type ParseNodeIDFn func(string) (roachpb.NodeID, bool, error)

// GetNodeIDHTTPAddressFn is a callback to look up HTTP addresses
// for a given NodeID. It exists to prevent a dependency on `gossip`.
type GetNodeIDHTTPAddressFn func(roachpb.NodeID) (*util.UnresolvedAddr, roachpb.Locality, error)

// nodeProxyHandler is expected to return responses that will be inspected
// by customers directly (for instance, by third party software that
// scrapes our metrics endpoint) and thus well-formed error responses
// are necessary.
func (np *nodeProxy) nodeProxyHandler(
	w http.ResponseWriter, r *http.Request, next http.HandlerFunc,
) {
	ctx := r.Context()
	nodeIDString, clearCookieOnError, err := np.getNodeIDFromRequest(r)
	if err != nil {
		if errors.Is(err, ErrNoNodeID) {
			next(w, r)
			return
		}
		if clearCookieOnError {
			resetCookie(w, r)
		}
		panic(errors.Wrap(err, "server: unexpected error reading nodeID from request"))
	}
	nodeID, local, err := np.parseNodeID(nodeIDString)
	if err != nil {
		httpErr := errors.Wrapf(err, "server: error parsing nodeID from request: %s", nodeIDString)
		log.Errorf(ctx, "%v", httpErr)
		if clearCookieOnError {
			resetCookie(w, r)
		}
		// Users are expected to recover from this by formatting their
		// nodeID properly, hence the 4xx code.
		http.Error(w, httpErr.Error(), http.StatusBadRequest)
	}
	if local {
		next(w, r)
		return
	}
	np.routeToNode(w, r, clearCookieOnError, nodeID)
}

// ErrNoNodeID is returned by getNodeIDFromRequest if the request
// contains no nodeID to proxy to.
var ErrNoNodeID = errors.New("http: nodeID not present in request")

func (np *nodeProxy) getNodeIDFromRequest(
	r *http.Request,
) (nodeIDString string, nodeIDFromCookie bool, retErr error) {
	queryParam := r.URL.Query().Get(RemoteNodeID)
	if queryParam != "" {
		return queryParam, false, nil
	}
	cookie, err := r.Cookie(RemoteNodeID)
	if err != nil {
		if errors.Is(err, http.ErrNoCookie) {
			return "", false, ErrNoNodeID
		}
		// This should never trigger based on the implementation of
		// `Cookie` in stdlib.
		return "", false, errors.Wrapf(err, "server: error decoding node routing cookie")
	}
	return cookie.Value, true, nil
}

// routeToNode will proxy the given request to the node identified by
// nodeID.a
func (np *nodeProxy) routeToNode(
	w http.ResponseWriter, r *http.Request, clearCookieOnError bool, nodeID roachpb.NodeID,
) {
	addr, _, err := np.getNodeIDHTTPAddress(nodeID)
	if err != nil {
		httpErr := errors.Wrapf(err, "unable to get address for n%d", nodeID)
		log.Errorf(r.Context(), "%v", httpErr)

		// Reset the cookie to `local` so the user session isn't stuck on a bad node
		if clearCookieOnError {
			resetCookie(w, r)
		}
		// Users could recover from this by asking for a nodeID the cluster
		// knows about, hence the 4xx code.
		http.Error(w, httpErr.Error(), http.StatusBadRequest)
		return
	}
	var forwardedUser *string
	if np.scheme == "https" {
		// The other node sees this node's certificate, which is root. Name
		// the request's own user, or none, so that it does not run as root.
		// A request without TLS, such as /health on the plain HTTP
		// listener, has no user.
		var user string
		if r.TLS != nil {
			if np.authServer == nil {
				http.Error(w, "server: HTTP authentication is not set up yet", http.StatusServiceUnavailable)
				return
			}
			if u, err := np.authServer.UserFromRequest(r); err == nil {
				user = u.Normalized()
			}
		}
		forwardedUser = &user
	}
	r = r.WithContext(context.WithValue(r.Context(), forwardedUserKey{}, forwardedUser))

	proxy, ok := np.proxyCache.get(*addr)
	if !ok {
		u := url.URL{
			Scheme: np.scheme,
			Host:   addr.AddressField,
		}
		// This client is initialized with the TLS config we require in
		// order to make requests to the HTTP server.
		httpClient, err := np.rpcContext.GetHTTPClient()
		if err != nil {
			if clearCookieOnError {
				resetCookie(w, r)
			}
			panic(errors.Wrapf(err, "server: failed to get httpClient"))
		}
		proxy = &httputil.ReverseProxy{
			// Rewrite runs after the proxy has removed the hop-by-hop
			// headers, so a client cannot make it drop the forwarded user
			// by naming that header in Connection.
			Rewrite: func(pr *httputil.ProxyRequest) {
				pr.SetURL(&u)
				pr.Out.Host = pr.In.Host
				pr.Out.Header.Del(authserver.ForwardedUserHeader)
				if user, _ := pr.In.Context().Value(forwardedUserKey{}).(*string); user != nil {
					pr.Out.Header[authserver.ForwardedUserHeader] = []string{*user}
				}
			},
			Transport: httpClient.Transport,
		}
		np.proxyCache.put(*addr, proxy)
	}
	proxy.ServeHTTP(w, r)
}

// forwardedUserKey is the context key of the user that routeToNode names in
// the forwarded request: nil when the proxy talks plain HTTP and presents no
// certificate.
type forwardedUserKey struct{}

// resetCookie only resets the cookie if one exists on the request
func resetCookie(w http.ResponseWriter, req *http.Request) {
	if _, err := req.Cookie(RemoteNodeID); err == nil {
		w.Header().Set("set-cookie", fmt.Sprintf("%s=;expires=Thu, 01 Jan 1970 00:00:01 GMT;path=/", RemoteNodeID))
	}
}

// reverseProxyCache implements a shared cache of `ReverseProxy`
// instances keyed by their addresses so that we can effectively
// re-use proxies when routing multiple requests to the same node.
type reverseProxyCache struct {
	mu            syncutil.RWMutex
	proxiesByAddr map[util.UnresolvedAddr]*httputil.ReverseProxy
}

func (c *reverseProxyCache) get(addr util.UnresolvedAddr) (*httputil.ReverseProxy, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	p, ok := c.proxiesByAddr[addr]
	return p, ok
}

func (c *reverseProxyCache) put(addr util.UnresolvedAddr, proxy *httputil.ReverseProxy) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.proxiesByAddr == nil {
		c.proxiesByAddr = make(map[util.UnresolvedAddr]*httputil.ReverseProxy)
	}
	c.proxiesByAddr[addr] = proxy
}
