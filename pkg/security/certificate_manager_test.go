// Copyright 2026 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package security_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/security"
	"github.com/cockroachdb/cockroach/pkg/security/certconfig"
	"github.com/cockroachdb/cockroach/pkg/security/securitytest"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/testutils"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/stop"
	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/require"
)

var nodeID = securitytest.Identity{SPIFFE: securitytest.SPIFFEID("node")}

// testPKI has separate node and client CAs. Node certs identify nodes by
// SPIFFE ID only: no CN and no DNS or IP name.
type testPKI struct {
	// nodeRoot signs nodeCA, which signs the node certs.
	nodeRoot, nodeCA, clientCA *securitytest.CA
	rules                      string
}

func newTestPKI(t *testing.T, seed int64) testPKI {
	g := securitytest.NewGenerator(seed, func(err error) { t.Fatal(err) })
	nodeRoot := g.RootCA("node CA")
	return testPKI{
		nodeRoot: nodeRoot,
		nodeCA:   nodeRoot.IntermediateCA("node intermediate"),
		clientCA: g.RootCA("client CA"),
		rules: "node spiffe=" + securitytest.SPIFFEID("node") + "\n" +
			"client cn=alice role=alice\n",
	}
}

func writeFile(t *testing.T, path string, b []byte) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, b, 0600))
}

// writeNode writes the files of a node with the given cert to dir and
// returns its config.
func (p testPKI) writeNode(t *testing.T, dir string, node securitytest.Pair) certconfig.Config {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0700))
	writeFile(t, filepath.Join(dir, "node.crt"), node.CertPEM)
	writeFile(t, filepath.Join(dir, "node.key"), node.KeyPEM)
	writeFile(t, filepath.Join(dir, "node-ca.crt"), p.nodeRoot.CertPEM)
	writeFile(t, filepath.Join(dir, "client-ca.crt"), p.clientCA.CertPEM)
	writeFile(t, filepath.Join(dir, "rules"), []byte(p.rules))
	return certconfig.Config{
		Cert:     filepath.Join(dir, "node.crt"),
		Key:      filepath.Join(dir, "node.key"),
		NodeCA:   filepath.Join(dir, "node-ca.crt"),
		ClientCA: filepath.Join(dir, "client-ca.crt"),
		Rules:    filepath.Join(dir, "rules"),
	}
}

func newManager(t *testing.T, cfg certconfig.Config) *security.CertificateManager {
	t.Helper()
	cm, err := security.NewCertificateManager(cfg, security.ClusterTLSSettings(cluster.MakeTestingClusterSettings()))
	require.NoError(t, err)
	return cm
}

// handshake runs a TLS handshake between a client and a server config over
// loopback TCP and returns the errors on each side. In TLS 1.3 the client
// finishes before the server has checked the client's certificate, so a
// rejection by the server shows only in serverErr.
func handshake(t *testing.T, client, server *tls.Config) (clientErr, serverErr error) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = ln.Close() }()
	errCh := make(chan error, 1)
	go func() {
		c, err := ln.Accept()
		if err != nil {
			errCh <- err
			return
		}
		defer c.Close()
		errCh <- tls.Server(c, server).Handshake()
	}()
	c, err := net.Dial("tcp", ln.Addr().String())
	require.NoError(t, err)
	defer c.Close()
	clientErr = tls.Client(c, client).Handshake()
	serverErr = <-errCh
	return clientErr, serverErr
}

func pairConfig(t *testing.T, p securitytest.Pair) tls.Certificate {
	pair, err := tls.X509KeyPair(p.CertPEM, p.KeyPEM)
	require.NoError(t, err)
	return pair
}

// presenting returns a client config that sends the cert whatever CAs the
// server asks for, and does not check the server.
func presenting(t *testing.T, p securitytest.Pair) *tls.Config {
	pair := pairConfig(t, p)
	return &tls.Config{
		InsecureSkipVerify: true,
		GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			return &pair, nil
		},
	}
}

// TestTwoTrustPools checks that the RPC port trusts only the node CA and the
// node rules, SQL only the client CA, and that a dialing node checks the
// server the same way, without a host name.
func TestTwoTrustPools(t *testing.T) {
	defer leaktest.AfterTest(t)()
	p := newTestPKI(t, 1)
	dir := t.TempDir()
	server := newManager(t, p.writeNode(t, filepath.Join(dir, "a"), p.nodeCA.NodeCert(nodeID)))
	other := newManager(t, p.writeNode(t, filepath.Join(dir, "b"), p.nodeCA.NodeCert(nodeID)))

	rpcServer, err := server.RPCServerTLSConfig()
	require.NoError(t, err)
	rpcClient, err := other.NodeClientTLSConfig()
	require.NoError(t, err)
	sqlServer, err := server.SQLServerTLSConfig()
	require.NoError(t, err)

	// Node to node, identified by SPIFFE ID only.
	cErr, sErr := handshake(t, rpcClient, rpcServer)
	require.NoError(t, cErr)
	require.NoError(t, sErr)

	// The node identity signed by the client CA is rejected when it dials in.
	wrongCA := p.clientCA.NodeCert(nodeID)
	_, sErr = handshake(t, presenting(t, wrongCA), rpcServer)
	require.Error(t, sErr)
	require.Contains(t, sErr.Error(), "verifying node certificate")

	// ...and when it serves: a dialing node rejects it.
	cErr, _ = handshake(t, rpcClient, &tls.Config{Certificates: []tls.Certificate{pairConfig(t, wrongCA)}})
	require.Error(t, cErr)
	require.Contains(t, cErr.Error(), "verifying node certificate")

	// A node CA cert that matches no node rule is rejected in both directions.
	stranger := p.nodeCA.NodeCert(securitytest.Identity{SPIFFE: securitytest.SPIFFEID("stranger")})
	_, sErr = handshake(t, presenting(t, stranger), rpcServer)
	require.ErrorContains(t, sErr, "matches no node rule")
	cErr, _ = handshake(t, rpcClient, &tls.Config{Certificates: []tls.Certificate{pairConfig(t, stranger)}})
	require.ErrorContains(t, cErr, "matches no node rule")

	// A node identity without server usage is not a node.
	clientUsage := p.nodeCA.ClientCert(nodeID)
	_, sErr = handshake(t, presenting(t, clientUsage), rpcServer)
	require.ErrorContains(t, sErr, "needs both server and client extended key usage")

	// Nor is a leaf that lists no usage, which Go's chain check alone would
	// accept for every use.
	noUsage := p.nodeCA.CertWithUsage(nodeID)
	_, sErr = handshake(t, presenting(t, noUsage), rpcServer)
	require.ErrorContains(t, sErr, "needs both server and client extended key usage")

	// Nor is a dual-use leaf under an intermediate restricted to client use.
	clientOnlyCA := p.nodeRoot.IntermediateCA("client-only intermediate", x509.ExtKeyUsageClientAuth)
	restricted := clientOnlyCA.NodeCert(nodeID)
	_, sErr = handshake(t, presenting(t, restricted), rpcServer)
	require.ErrorContains(t, sErr, "needs both server and client extended key usage")

	// A SQL client cert from the client CA passes TLS and maps to its role.
	alice := p.clientCA.ClientCert(securitytest.Identity{CN: "alice"})
	sqlClient := presenting(t, alice)
	cErr, sErr = handshake(t, sqlClient, sqlServer)
	require.NoError(t, cErr)
	require.NoError(t, sErr)
	roles, err := server.ClientRoles(certChain(t, alice))
	require.NoError(t, err)
	require.Equal(t, "alice", roles[0].Normalized())

	// The same client identity signed by the node CA is rejected by SQL, and
	// a client cert is rejected on the RPC port.
	aliceWrongCA := p.nodeCA.ClientCert(securitytest.Identity{CN: "alice"})
	_, sErr = handshake(t, presenting(t, aliceWrongCA), sqlServer)
	require.ErrorContains(t, sErr, "certificate signed by unknown authority")
	_, err = server.ClientRoles(certChain(t, aliceWrongCA))
	require.Error(t, err)
	_, sErr = handshake(t, sqlClient, rpcServer)
	require.Error(t, sErr)

	// A client CA cert with no client rule passes TLS but maps to no role.
	bob := p.clientCA.ClientCert(securitytest.Identity{CN: "bob"})
	_, err = server.ClientRoles(certChain(t, bob))
	require.ErrorContains(t, err, "matches no client rule")
}

func certChain(t *testing.T, p securitytest.Pair) []*x509.Certificate {
	certs, err := security.PEMContentsToX509(p.CertPEM)
	require.NoError(t, err)
	return certs
}

func TestLoadChecks(t *testing.T) {
	defer leaktest.AfterTest(t)()
	p := newTestPKI(t, 2)
	dir := t.TempDir()
	cfg := p.writeNode(t, dir, p.nodeCA.NodeCert(nodeID))

	// A node cert needs server and client usage.
	clientOnly := p.nodeCA.ClientCert(nodeID)
	writeFile(t, cfg.Cert, clientOnly.CertPEM)
	writeFile(t, cfg.Key, clientOnly.KeyPEM)
	_, err := security.NewCertificateManager(cfg, security.ClusterTLSSettings(cluster.MakeTestingClusterSettings()))
	require.ErrorContains(t, err, "needs both server and client extended key usage")
	require.True(t, errors.Is(err, security.ErrCertManagement))

	// A node cert must chain to the node CA.
	wrongCA := p.clientCA.NodeCert(nodeID)
	writeFile(t, cfg.Cert, wrongCA.CertPEM)
	writeFile(t, cfg.Key, wrongCA.KeyPEM)
	_, err = security.NewCertificateManager(cfg, security.ClusterTLSSettings(cluster.MakeTestingClusterSettings()))
	require.ErrorContains(t, err, "does not verify against node CA")

	// A CA bundle whose second root is cut off does not load.
	writeFile(t, cfg.NodeCA, append(append([]byte(nil), p.nodeRoot.CertPEM...), p.clientCA.CertPEM[:100]...))
	_, err = security.NewCertificateManager(cfg, security.ClusterTLSSettings(cluster.MakeTestingClusterSettings()))
	require.ErrorContains(t, err, "PEM blocks are not complete")
	writeFile(t, cfg.NodeCA, p.nodeRoot.CertPEM)

	// The key file must not be readable by others.
	good := p.nodeCA.NodeCert(nodeID)
	writeFile(t, cfg.Cert, good.CertPEM)
	writeFile(t, cfg.Key, good.KeyPEM)
	require.NoError(t, os.Chmod(cfg.Key, 0644))
	_, err = security.NewCertificateManager(cfg, security.ClusterTLSSettings(cluster.MakeTestingClusterSettings()))
	require.ErrorContains(t, err, "exceeds")
}

// TestReload checks that the files reload on file events and on SIGHUP,
// that a set is swapped in only when all of it loads, and that outgoing
// client certs reload too.
func TestReload(t *testing.T) {
	defer leaktest.AfterTest(t)()
	ctx := context.Background()
	stopper := stop.NewStopper()
	defer stopper.Stop(ctx)

	p := newTestPKI(t, 3)
	certs := []securitytest.Pair{p.nodeCA.NodeCert(nodeID)}
	for _, h := range []string{"v2", "v3"} {
		// Each cert differs by its host name.
		certs = append(certs, p.nodeCA.NodeCert(nodeID, h))
	}

	// Kubernetes layout: dir/..data points at a versioned directory and
	// each file is a symlink through ..data.
	dir := t.TempDir()
	versioned := func(name string) string { return filepath.Join(dir, name) }
	p.writeNode(t, versioned("..v1"), certs[0])
	require.NoError(t, os.Symlink("..v1", versioned("..data")))
	for _, f := range []string{"node.crt", "node.key", "node-ca.crt", "client-ca.crt", "rules"} {
		require.NoError(t, os.Symlink(filepath.Join("..data", f), versioned(f)))
	}
	cfg := certconfig.Config{
		Cert: versioned("node.crt"), Key: versioned("node.key"),
		NodeCA: versioned("node-ca.crt"), ClientCA: versioned("client-ca.crt"),
		Rules: versioned("rules"),
	}
	cm := newManager(t, cfg)
	require.NoError(t, cm.Watch(ctx, stopper))
	clientCfg, err := cm.NodeClientTLSConfig()
	require.NoError(t, err)

	waitForCert := func(want securitytest.Pair) {
		t.Helper()
		wantLeaf := certChain(t, want)[0]
		testutils.SucceedsSoon(t, func() error {
			if !cm.NodeCert().Equal(wantLeaf) {
				return errors.New("node cert not reloaded yet")
			}
			// Outgoing connections present the new cert too.
			c, err := clientCfg.GetClientCertificate(&tls.CertificateRequestInfo{})
			if err != nil {
				return err
			}
			if !c.Leaf.Equal(wantLeaf) {
				return errors.New("client cert not reloaded yet")
			}
			return nil
		})
	}
	failures := func() int64 { return cm.Metrics().ReloadFailures.Count() }

	// A Kubernetes-style symlink swap.
	p.writeNode(t, versioned("..v2"), certs[1])
	require.NoError(t, os.Symlink("..v2", versioned("..data_tmp")))
	require.NoError(t, os.Rename(versioned("..data_tmp"), versioned("..data")))
	waitForCert(certs[1])

	// Replace the symlinks with plain files. A cert written before its key
	// fails to load, so the old set stays until the key arrives.
	for _, f := range []string{"node.crt", "node.key", "node-ca.crt", "client-ca.crt", "rules"} {
		b, err := os.ReadFile(versioned(f))
		require.NoError(t, err)
		require.NoError(t, os.Remove(versioned(f)))
		writeFile(t, versioned(f), b)
	}
	waitForCert(certs[1])
	before := failures()
	writeFile(t, cfg.Cert, certs[2].CertPEM)
	testutils.SucceedsSoon(t, func() error {
		if failures() == before {
			return errors.New("no failed reload yet")
		}
		return nil
	})
	require.True(t, cm.NodeCert().Equal(certChain(t, certs[1])[0]), "old cert kept")
	writeFile(t, cfg.Key, certs[2].KeyPEM)
	waitForCert(certs[2])

	// A broken CA file keeps the old set.
	before = failures()
	writeFile(t, cfg.NodeCA, []byte("not a certificate"))
	testutils.SucceedsSoon(t, func() error {
		if failures() == before {
			return errors.New("no failed reload yet")
		}
		return nil
	})
	rpcServer, err := cm.RPCServerTLSConfig()
	require.NoError(t, err)
	cErr, sErr := handshake(t, clientCfg, rpcServer)
	require.NoError(t, cErr, "the old CA still verifies")
	require.NoError(t, sErr)
	writeFile(t, cfg.NodeCA, p.nodeRoot.CertPEM)

	// A node CA bundle with two roots, then a reload where the second root
	// is complete PEM but not a certificate: both old roots stay trusted.
	secondRoot := p.clientCA
	writeFile(t, cfg.NodeCA, append(append([]byte(nil), p.nodeRoot.CertPEM...), secondRoot.CertPEM...))
	secondRootNode := certChain(t, secondRoot.NodeCert(nodeID))
	testutils.SucceedsSoon(t, func() error {
		return cm.VerifyNode(secondRootNode, x509.ExtKeyUsageClientAuth)
	})
	before = failures()
	broken := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte("not DER")})
	writeFile(t, cfg.NodeCA, append(append([]byte(nil), p.nodeRoot.CertPEM...), broken...))
	testutils.SucceedsSoon(t, func() error {
		if failures() == before {
			return errors.New("no failed reload yet")
		}
		return nil
	})
	require.NoError(t, cm.VerifyNode(secondRootNode, x509.ExtKeyUsageClientAuth), "the second root is still trusted")
	cErr, sErr = handshake(t, clientCfg, rpcServer)
	require.NoError(t, cErr, "the first root still verifies")
	require.NoError(t, sErr)
	writeFile(t, cfg.NodeCA, p.nodeRoot.CertPEM)

	// A cert chain whose intermediate is complete PEM but not a certificate
	// keeps the old node cert.
	before = failures()
	writeFile(t, cfg.Cert, append(append([]byte(nil), certs[2].CertPEM...), broken...))
	testutils.SucceedsSoon(t, func() error {
		if failures() == before {
			return errors.New("no failed reload yet")
		}
		return nil
	})
	require.True(t, cm.NodeCert().Equal(certChain(t, certs[2])[0]), "old cert kept")
	writeFile(t, cfg.Cert, certs[2].CertPEM)

	// So does a broken rules file.
	before = failures()
	writeFile(t, cfg.Rules, []byte("bogus line\n"))
	testutils.SucceedsSoon(t, func() error {
		if failures() == before {
			return errors.New("no failed reload yet")
		}
		return nil
	})
	require.True(t, cm.Rules().MatchesNode(certChain(t, certs[2])[0]), "old rules kept")
	writeFile(t, cfg.Rules, []byte(p.rules))

	// SIGHUP reloads too. With a broken file, each reload fails: wait for
	// the reload of the file event, then check that SIGHUP adds another.
	waitForCert(certs[2])
	before = failures()
	writeFile(t, cfg.Rules, []byte("bogus line\n"))
	testutils.SucceedsSoon(t, func() error {
		if failures() == before {
			return errors.New("no failed reload yet")
		}
		return nil
	})
	before = failures()
	require.NoError(t, syscall.Kill(os.Getpid(), syscall.SIGHUP))
	testutils.SucceedsSoon(t, func() error {
		if failures() == before {
			return errors.New("SIGHUP did not reload")
		}
		return nil
	})
}
