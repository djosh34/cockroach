// Copyright 2026 The Cockroach Authors.
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
	"io"
	"net/http"
	"path/filepath"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/security/certconfig"
	"github.com/cockroachdb/cockroach/pkg/security/securityassets"
	"github.com/cockroachdb/cockroach/pkg/server/apiconstants"
	"github.com/cockroachdb/cockroach/pkg/server/authserver"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/stretchr/testify/require"
)

// TestHTTPCertAuth checks who the HTTP API admits: /health and
// /_status/vars are open, a node cert is root, a client cert is the role
// of its client rule with that role's privileges, and a request that one
// node forwards to another keeps its own user.
func TestHTTPCertAuth(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	ctx := context.Background()
	tc := serverutils.StartCluster(t, 2, base.TestClusterArgs{
		ServerArgs: base.TestServerArgs{
			DefaultTestTenant: base.TestIsSpecificToStorageLayerAndNeedsASystemTenant,
		},
	})
	defer tc.Stopper().Stop(ctx)
	ts := tc.Server(0)
	// NOSQLLOGIN blocks SQL only: testuser still uses the HTTP API.
	_, err := tc.ServerConn(0).Exec("CREATE USER testuser NOSQLLOGIN")
	require.NoError(t, err)

	read := func(name string) []byte {
		b, err := securityassets.GetLoader().ReadFile(filepath.Join(certconfig.EmbeddedDir, name))
		require.NoError(t, err)
		return b
	}
	pool := x509.NewCertPool()
	require.True(t, pool.AppendCertsFromPEM(read("ca.crt")))
	client := func(certFile string) *http.Client {
		cfg := &tls.Config{RootCAs: pool}
		if certFile != "" {
			pair, err := tls.X509KeyPair(read(certFile+".crt"), read(certFile+".key"))
			require.NoError(t, err)
			cfg.Certificates = []tls.Certificate{pair}
		}
		return &http.Client{Transport: &http.Transport{TLSClientConfig: cfg}}
	}
	get := func(c *http.Client, path string, header map[string]string) int {
		req, err := http.NewRequest("GET", ts.AdminURL().WithPath(path).String(), nil)
		require.NoError(t, err)
		for k, v := range header {
			req.Header[http.CanonicalHeaderKey(k)] = []string{v}
		}
		resp, err := c.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		_, err = io.Copy(io.Discard, resp.Body)
		require.NoError(t, err)
		return resp.StatusCode
	}

	none := client("")
	root := client("client.root")
	testuser := client("client.testuser")
	node := client("node")
	// The v2 sessions endpoint needs the admin role.
	adminOnly := apiconstants.APIV2Path + "sessions/"
	// Read on node 2 through node 1.
	remote := "/_status/nodes?" + RemoteNodeID + "=2"
	forward := func(user string) map[string]string {
		return map[string]string{authserver.ForwardedUserHeader: user}
	}

	for _, c := range []struct {
		name   string
		client *http.Client
		path   string
		header map[string]string
		want   int
	}{
		{"health without cert", none, "/health", nil, http.StatusOK},
		{"vars without cert", none, "/_status/vars", nil, http.StatusOK},
		{"api without cert", none, adminOnly, nil, http.StatusUnauthorized},
		{"v1 api without cert", none, "/_status/nodes", nil, http.StatusUnauthorized},
		{"root client cert", root, adminOnly, nil, http.StatusOK},
		{"node cert", node, adminOnly, nil, http.StatusOK},
		{"non-admin client cert", testuser, adminOnly, nil, http.StatusForbidden},
		{"client cert cannot claim another user", testuser, adminOnly, forward("root"), http.StatusForbidden},
		{"node cert forwarding a non-admin", node, adminOnly, forward("testuser"), http.StatusForbidden},
		{"node cert forwarding no user", node, adminOnly, forward(""), http.StatusUnauthorized},
		{"root through another node", root, remote, nil, http.StatusOK},
		{"non-admin through another node", testuser, remote, nil, http.StatusForbidden},
		// The proxy drops headers that Connection names; the forwarded user
		// must survive that.
		{"hop-by-hop header through another node", testuser, remote,
			map[string]string{"Connection": authserver.ForwardedUserHeader}, http.StatusForbidden},
		{"no cert through another node", none, remote, nil, http.StatusUnauthorized},
	} {
		t.Run(c.name, func(t *testing.T) {
			require.Equal(t, c.want, get(c.client, c.path, c.header))
		})
	}
}
