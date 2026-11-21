// Copyright 2026 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package server_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/security/certconfig"
	"github.com/cockroachdb/cockroach/pkg/security/securitytest"
	"github.com/cockroachdb/cockroach/pkg/testutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/testcluster"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

// TestSPIFFEIdentityCluster runs a 3-node cluster whose nodes are
// identified by SPIFFE ID only, with separate node and client CAs. It
// checks that SQL clients connect with sslmode=verify-full and a client
// cert, that certs from the wrong CA are refused on both ports, and that
// nodes report rules that differ between them.
func TestSPIFFEIdentityCluster(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)
	ctx := context.Background()

	g := securitytest.NewGenerator(11)
	nodeRoot := g.RootCA("node CA")
	nodeCA := nodeRoot.IntermediateCA("node intermediate")
	clientCA := g.RootCA("client CA")
	rules := ""
	for i := 1; i <= 3; i++ {
		rules += fmt.Sprintf("node spiffe=%s\n", securitytest.SPIFFEID(fmt.Sprintf("node/%d", i)))
	}
	rules += "client cn=root role=root\nclient spiffe=" + securitytest.SPIFFEID("app") + " role=app\n"

	dir := t.TempDir()
	write := func(path string, b []byte) {
		require.NoError(t, os.WriteFile(path, b, 0600))
	}
	write(filepath.Join(dir, "node-ca.crt"), nodeRoot.CertPEM)
	write(filepath.Join(dir, "client-ca.crt"), clientCA.CertPEM)
	perNode := map[int]base.TestServerArgs{}
	var rulesFiles []string
	for i := 0; i < 3; i++ {
		nodeDir := filepath.Join(dir, fmt.Sprint(i+1))
		require.NoError(t, os.Mkdir(nodeDir, 0700))
		// No CN: the SPIFFE ID identifies the node. The host names are
		// for SQL and HTTP clients, which check them.
		cert := nodeCA.NodeCert(securitytest.Identity{
			SPIFFE: securitytest.SPIFFEID(fmt.Sprintf("node/%d", i+1)),
		}, "localhost", "127.0.0.1")
		write(filepath.Join(nodeDir, "node.crt"), cert.CertPEM)
		write(filepath.Join(nodeDir, "node.key"), cert.KeyPEM)
		rulesFile := filepath.Join(nodeDir, "rules")
		write(rulesFile, []byte(rules))
		rulesFiles = append(rulesFiles, rulesFile)
		perNode[i] = base.TestServerArgs{
			Certs: certconfig.Config{
				Cert:     filepath.Join(nodeDir, "node.crt"),
				Key:      filepath.Join(nodeDir, "node.key"),
				NodeCA:   filepath.Join(dir, "node-ca.crt"),
				ClientCA: filepath.Join(dir, "client-ca.crt"),
				Rules:    rulesFile,
			},
		}
	}

	// Starting the cluster waits for full replication, which needs every
	// node to reach the others.
	tc := testcluster.StartTestCluster(t, 3, base.TestClusterArgs{
		ServerArgs: base.TestServerArgs{
			DefaultTestTenant: base.TestIsSpecificToStorageLayerAndNeedsASystemTenant,
		},
		ServerArgsPerNode: perNode,
	})
	defer tc.Stopper().Stop(ctx)

	// SQL with verify-full and a client cert.
	connect := func(srv int, client securitytest.Pair, user string) (*pgx.Conn, error) {
		clientDir := t.TempDir()
		write(filepath.Join(clientDir, "client.crt"), client.CertPEM)
		write(filepath.Join(clientDir, "client.key"), client.KeyPEM)
		host, port, err := net.SplitHostPort(tc.Server(srv).AdvSQLAddr())
		require.NoError(t, err)
		u := url.URL{
			Scheme: "postgresql",
			User:   url.User(user),
			Host:   net.JoinHostPort(host, port),
			RawQuery: url.Values{
				"sslmode":     {"verify-full"},
				"sslrootcert": {filepath.Join(dir, "node-ca.crt")},
				"sslcert":     {filepath.Join(clientDir, "client.crt")},
				"sslkey":      {filepath.Join(clientDir, "client.key")},
			}.Encode(),
		}
		return pgx.Connect(ctx, u.String())
	}
	root, err := connect(0, clientCA.ClientCert(securitytest.Identity{CN: "root"}), "root")
	require.NoError(t, err)
	defer func() { _ = root.Close(ctx) }()
	_, err = root.Exec(ctx, "CREATE USER app")
	require.NoError(t, err)

	app, err := connect(2, clientCA.ClientCert(securitytest.Identity{SPIFFE: securitytest.SPIFFEID("app")}), "app")
	require.NoError(t, err)
	var user string
	require.NoError(t, app.QueryRow(ctx, "SELECT current_user").Scan(&user))
	require.Equal(t, "app", user)
	require.NoError(t, app.Close(ctx))

	// A client cert may only log in as the role its rule names.
	_, err = connect(1, clientCA.ClientCert(securitytest.Identity{SPIFFE: securitytest.SPIFFEID("app")}), "root")
	require.ErrorContains(t, err, "no client rule maps certificate")

	// The right client identity signed by the node CA is refused by SQL.
	_, err = connect(1, nodeCA.ClientCert(securitytest.Identity{CN: "root"}), "root")
	require.Error(t, err)

	// The right node identity signed by the client CA is refused on the
	// RPC port.
	wrongCA := clientCA.NodeCert(securitytest.Identity{SPIFFE: securitytest.SPIFFEID("node/1")})
	pair, err := tls.X509KeyPair(wrongCA.CertPEM, wrongCA.KeyPEM)
	require.NoError(t, err)
	conn, err := tls.Dial("tcp", tc.Server(1).AdvRPCAddr(), &tls.Config{
		InsecureSkipVerify: true,
		GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			return &pair, nil
		},
	})
	if err == nil {
		// In TLS 1.3 the server's rejection arrives on the first read.
		_, err = conn.Read(make([]byte, 1))
		_ = conn.Close()
	}
	require.Error(t, err)

	// Different rules on node 3 are reported by every node, and the report
	// clears once the files match again. The files reload on their own.
	mismatched := func(srv int) int64 {
		cm, err := tc.Server(srv).RPCContext().GetCertificateManager()
		require.NoError(t, err)
		return cm.Metrics().RulesMismatchedNodes.Value()
	}
	waitFor := func(want []int64) {
		testutils.SucceedsSoon(t, func() error {
			for i, w := range want {
				if got := mismatched(i); got != w {
					return errors.Newf("n%d: %d mismatched nodes, want %d", i+1, got, w)
				}
			}
			return nil
		})
	}
	waitFor([]int64{0, 0, 0})
	write(rulesFiles[2], []byte(rules+"client cn=extra role=extra\n"))
	waitFor([]int64{1, 1, 2})
	write(rulesFiles[2], []byte(rules))
	waitFor([]int64{0, 0, 0})
}
