// Copyright 2026 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package certconfig_test

import (
	"crypto/x509"
	"encoding/pem"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/security/certconfig"
	"github.com/cockroachdb/cockroach/pkg/security/securitytest"
	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/stretchr/testify/require"
)

func parseLeaf(t *testing.T, p securitytest.Pair) *x509.Certificate {
	block, _ := pem.Decode(p.CertPEM)
	require.NotNil(t, block)
	c, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)
	return c
}

func TestParseRulesErrors(t *testing.T) {
	defer leaktest.AfterTest(t)()
	for _, tc := range []struct {
		line string
		err  string
	}{
		{`peer cn=a`, `must start with node or client`},
		{`node`, `needs cn, spiffe or both`},
		{`node cn`, `is not key=value`},
		{`node cn=`, `is not key=value`},
		{`node cn=a cn=b`, `key "cn" given twice`},
		{`node cn=a role=b`, `a node line has no role`},
		{`node spiffe=http://a`, `does not start with spiffe://`},
		{`node cn=a ou=b`, `unknown key "ou"`},
		{`client cn=a`, `a client line needs a role`},
		{`client role=a`, `needs cn, spiffe or both`},
		{`client cn=a role=node`, `role node is reserved`},
		{`client cn="a role=b`, `bad quoted value`},
		{`client cn="a"b role=b`, `text after quoted value`},
	} {
		t.Run(tc.line, func(t *testing.T) {
			_, err := certconfig.ParseRules([]byte("# comment\n\n" + tc.line + "\n"))
			require.Error(t, err)
			require.Regexp(t, tc.err, err)
			require.Regexp(t, "rules line 3", err)
		})
	}
}

func TestRulesMatch(t *testing.T) {
	defer leaktest.AfterTest(t)()
	g := securitytest.NewGenerator(7, func(err error) { t.Fatal(err) })
	ca := g.RootCA("ca")
	cert := func(cn, spiffe string) *x509.Certificate {
		return parseLeaf(t, ca.ClientCert(securitytest.Identity{CN: cn, SPIFFE: spiffe}))
	}
	app := securitytest.SPIFFEID("app")

	rules, err := certconfig.ParseRules([]byte(`
node cn=n1
node spiffe=` + securitytest.SPIFFEID("node") + `
client cn=alice role=alice
client spiffe=` + app + ` role=app
client cn=bob spiffe=` + app + ` role=bob
client cn="Jane Doe" role=jane
client cn=alice role=reader
`))
	require.NoError(t, err)

	roles := func(c *x509.Certificate) []string {
		var out []string
		for _, r := range rules.ClientRoles(c) {
			out = append(out, r.Normalized())
		}
		return out
	}

	require.True(t, rules.MatchesNode(cert("n1", "")))
	require.True(t, rules.MatchesNode(cert("", securitytest.SPIFFEID("node"))))
	require.True(t, rules.MatchesNode(cert("other", securitytest.SPIFFEID("node"))))
	require.False(t, rules.MatchesNode(cert("n2", "")))
	require.False(t, rules.MatchesNode(cert("N1", "")), "matching is exact")
	require.False(t, rules.MatchesNode(cert("alice", "")), "a client line is not a node line")

	require.Equal(t, []string{"alice", "reader"}, roles(cert("alice", "")))
	require.Equal(t, []string{"app"}, roles(cert("x", app)))
	require.Equal(t, []string{"app", "bob"}, roles(cert("bob", app)))
	require.Empty(t, roles(cert("bob", "")), "a line with two keys needs both")
	require.Equal(t, []string{"jane"}, roles(cert("Jane Doe", "")))
	require.Empty(t, roles(cert("n1", "")), "a node line is not a client line")

	// A certificate with two URIs has no SPIFFE ID.
	twoURIs := cert("x", app)
	other, err := url.Parse(securitytest.SPIFFEID("other"))
	require.NoError(t, err)
	twoURIs.URIs = append(twoURIs.URIs, other)
	require.Empty(t, roles(twoURIs))

	// Lines round-trips through ParseRules.
	again, err := certconfig.ParseRules([]byte(joinLines(rules.Lines())))
	require.NoError(t, err)
	require.Equal(t, rules.Lines(), again.Lines())
	require.Contains(t, rules.Lines(), `client cn="Jane Doe" role=jane`)
}

func TestRulesAcceptTestUsers(t *testing.T) {
	defer leaktest.AfterTest(t)()
	for _, u := range securitytest.TestUsers {
		rules, err := certconfig.ParseRules([]byte("client cn=" + u + " role=" + u))
		require.NoError(t, err)
		require.Equal(t, []string{"client cn=" + u + " role=" + username.MakeSQLUsernameFromPreNormalizedString(u).Normalized()}, rules.Lines())
	}
}

func joinLines(lines []string) string {
	var s string
	for _, l := range lines {
		s += l + "\n"
	}
	return s
}

func TestDefaultClientUser(t *testing.T) {
	defer leaktest.AfterTest(t)()
	g := securitytest.NewGenerator(8, func(err error) { t.Fatal(err) })
	ca := g.RootCA("ca")
	dir := t.TempDir()
	write := func(name string, b []byte) string {
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, b, 0600))
		return path
	}
	c := certconfig.Config{
		ClientCert: write("client.crt", ca.ClientCert(securitytest.Identity{CN: "tool"}).CertPEM),
		Rules:      write("rules", []byte("client cn=other role=other\nclient cn=tool role=app\nclient cn=tool role=reader\n")),
	}
	user, err := certconfig.DefaultClientUser(c)
	require.NoError(t, err)
	require.Equal(t, "app", user, "the first matching rule")

	write("rules", []byte("client cn=other role=other\n"))
	user, err = certconfig.DefaultClientUser(c)
	require.NoError(t, err)
	require.Equal(t, "root", user, "no matching rule")

	c.ClientCert = ""
	user, err = certconfig.DefaultClientUser(c)
	require.NoError(t, err)
	require.Equal(t, "root", user, "no client cert")
}
