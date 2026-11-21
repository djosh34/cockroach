// Copyright 2026 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package securitytest_test

import (
	"crypto/x509"
	"encoding/pem"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/security/securitytest"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/stretchr/testify/require"
)

func makePKI(t *testing.T, seed int64) [][]byte {
	g := securitytest.NewGenerator(seed, func(err error) { t.Fatal(err) })
	root := g.RootCA("root")
	inter := root.IntermediateCA("intermediate")
	node := inter.NodeCert(securitytest.Identity{CN: "node", SPIFFE: securitytest.SPIFFEID("node")}, "localhost", "127.0.0.1")
	client := inter.ClientCert(securitytest.Identity{CN: "alice"})
	return [][]byte{root.CertPEM, inter.CertPEM, node.CertPEM, node.KeyPEM, client.CertPEM, client.KeyPEM}
}

// TestSameSeedSameCerts checks that a seed gives byte-identical certificates
// and keys, and another seed gives others.
func TestSameSeedSameCerts(t *testing.T) {
	defer leaktest.AfterTest(t)()
	require.Equal(t, makePKI(t, 1), makePKI(t, 1))
	a, b := makePKI(t, 1), makePKI(t, 2)
	for i := range a {
		require.NotEqual(t, a[i], b[i])
	}
}

func TestCertsVerify(t *testing.T) {
	defer leaktest.AfterTest(t)()
	g := securitytest.NewGenerator(3, func(err error) { t.Fatal(err) })
	root := g.RootCA("root")
	inter := root.IntermediateCA("intermediate")
	node := inter.NodeCert(securitytest.Identity{SPIFFE: securitytest.SPIFFEID("node")}, "localhost", "::1")

	var certs []*x509.Certificate
	for rest := node.CertPEM; ; {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		c, err := x509.ParseCertificate(block.Bytes)
		require.NoError(t, err)
		certs = append(certs, c)
	}
	require.Len(t, certs, 2, "leaf then intermediate")
	leaf := certs[0]
	require.Empty(t, leaf.Subject.CommonName)
	require.Equal(t, securitytest.SPIFFEID("node"), leaf.URIs[0].String())
	require.Equal(t, []string{"localhost"}, leaf.DNSNames)
	require.Len(t, leaf.IPAddresses, 1)

	roots := x509.NewCertPool()
	roots.AddCert(root.Cert)
	inters := x509.NewCertPool()
	inters.AddCert(certs[1])
	for _, usage := range []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth} {
		_, err := leaf.Verify(x509.VerifyOptions{
			Roots: roots, Intermediates: inters, KeyUsages: []x509.ExtKeyUsage{usage}, DNSName: "localhost",
		})
		require.NoError(t, err)
	}
}
