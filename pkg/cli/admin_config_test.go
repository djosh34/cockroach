// Copyright 2026 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package cli

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/security/certconfig"
	"github.com/cockroachdb/cockroach/pkg/security/securitytest"
	"github.com/cockroachdb/cockroach/pkg/testutils/testcluster"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/stretchr/testify/require"
)

// TestAdminCommandsWithConfig runs the admin commands with only --config,
// against nodes whose node CA and client CA differ: the commands present
// the node cert for RPCs and the config's client cert for SQL.
func TestAdminCommandsWithConfig(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)
	ctx := context.Background()

	g := securitytest.NewGenerator(21, func(err error) { t.Fatal(err) })
	nodeCA := g.RootCA("node CA")
	clientCA := g.RootCA("client CA")
	dir := t.TempDir()
	write := func(name string, b []byte) string {
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, b, 0600))
		return path
	}
	node := nodeCA.NodeCert(securitytest.Identity{SPIFFE: securitytest.SPIFFEID("node")}, "localhost", "127.0.0.1")
	client := clientCA.ClientCert(securitytest.Identity{CN: "admin-tool"})
	write("node.crt", node.CertPEM)
	write("node.key", node.KeyPEM)
	write("node-ca.crt", nodeCA.CertPEM)
	write("client-ca.crt", clientCA.CertPEM)
	write("client.crt", client.CertPEM)
	write("client.key", client.KeyPEM)
	// The client cert maps to admintool first, so commands connect as
	// admintool unless --user names root.
	write("rules", []byte("node spiffe="+securitytest.SPIFFEID("node")+"\n"+
		"client cn=admin-tool role=admintool\nclient cn=admin-tool role=root\n"))
	configFile := write("config.yaml", []byte("cert: node.crt\nkey: node.key\n"+
		"node-ca: node-ca.crt\nclient-ca: client-ca.crt\nrules: rules\n"+
		"client-cert: client.crt\nclient-key: client.key\n"))
	certs, err := certconfig.Load(configFile)
	require.NoError(t, err)

	tc := testcluster.StartTestCluster(t, 3, base.TestClusterArgs{
		ServerArgs: base.TestServerArgs{
			DefaultTestTenant: base.TestIsSpecificToStorageLayerAndNeedsASystemTenant,
			Certs:             certs,
		},
	})
	defer tc.Stopper().Stop(ctx)

	runArgs := func(args ...string) (string, error) {
		var runErr error
		out, err := captureOutput(func() {
			// The defaults pick up the captured stdout.
			TestingReset()
			runErr = Run(args)
		})
		require.NoError(t, err)
		return out, runErr
	}
	run := func(args ...string) (string, error) {
		addr := tc.Server(0).AdvRPCAddr()
		isSQL, err := isSQLCommand(args)
		require.NoError(t, err)
		if isSQL {
			addr = tc.Server(0).AdvSQLAddr()
		}
		return runArgs(append(args, "--host="+addr, "--config="+configFile)...)
	}

	_, err = run("sql", "--user=root", "-e", "CREATE USER admintool; GRANT admin TO admintool")
	require.NoError(t, err)

	// The cluster is up already, so init reaches the node and is refused.
	_, err = run("init")
	require.ErrorContains(t, err, "already been initialized")

	out, err := run("node", "ls")
	require.NoError(t, err)
	require.Equal(t, "id\n1\n2\n3\n", out)

	out, err = run("node", "status")
	require.NoError(t, err)
	require.Len(t, strings.Split(strings.TrimSpace(out), "\n"), 4, "a header and 3 nodes")

	// sql --config connects with the config's client cert, as the role of
	// the first rule that matches it.
	out, err = run("sql", "-e", "SELECT current_user")
	require.NoError(t, err)
	require.Equal(t, "current_user\nadmintool\n", out)

	// A URL with the cert files works too, without --config.
	u := "postgresql://root@" + tc.Server(0).AdvSQLAddr() + "/defaultdb?sslmode=verify-full" +
		"&sslrootcert=" + filepath.Join(dir, "node-ca.crt") +
		"&sslcert=" + filepath.Join(dir, "client.crt") +
		"&sslkey=" + filepath.Join(dir, "client.key")
	out, err = runArgs("sql", "--url", u, "-e", "SELECT current_user")
	require.NoError(t, err)
	require.Equal(t, "current_user\nroot\n", out)

	zipFile := filepath.Join(dir, "debug.zip")
	_, err = run("debug", "zip", zipFile, "--concurrency=1")
	require.NoError(t, err)
	r, err := zip.OpenReader(zipFile)
	require.NoError(t, err)
	defer func() { require.NoError(t, r.Close()) }()
	var sqlDump bool
	for _, f := range r.File {
		if strings.HasSuffix(f.Name, "crdb_internal.cluster_settings.txt") {
			sqlDump = true
		}
		require.False(t, strings.HasSuffix(f.Name, "crdb_internal.cluster_settings.txt.err.txt"),
			"the SQL dump failed: %s", f.Name)
	}
	require.True(t, sqlDump, "the zip has no SQL table dump")

	_, err = run("node", "drain", "3")
	require.NoError(t, err)
	// Three nodes cannot hold three replicas once one leaves, so skip the
	// readiness checks.
	_, err = run("node", "decommission", "3", "--wait=none", "--checks=skip")
	require.NoError(t, err)
	out, err = run("node", "status", "--decommission")
	require.NoError(t, err)
	require.Contains(t, out, "decommissioning")
}
