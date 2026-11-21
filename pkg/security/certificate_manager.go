// Copyright 2017 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package security

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"os"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/cockroachdb/cockroach/pkg/security/certconfig"
	"github.com/cockroachdb/cockroach/pkg/security/securityassets"
	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/cockroach/pkg/util/envutil"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/log/eventpb"
	"github.com/cockroachdb/cockroach/pkg/util/syncutil"
	"github.com/cockroachdb/cockroach/pkg/util/sysutil"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
	"github.com/cockroachdb/errors"
)

// CertificateManager holds the certificates, keys, CAs and identity rules
// named by the --config file, and builds the TLS configs from them.
//
// There are two trust pools. The node CA verifies nodes, on the RPC port
// and when a node dials another node. The client CA verifies SQL clients.
// A peer is admitted only if its certificate also matches a rule.
//
// Reload swaps in a new set of files only if every file parses and the keys
// match their certs. The TLS configs read the current set on every
// handshake, so a reload applies to new connections in both directions.
type CertificateManager struct {
	cfg         certconfig.Config
	tlsSettings TLSSettings
	certMetrics Metrics

	// Client cert expiration cache.
	clientCertExpirationCache *ClientCertExpirationCache

	// reloadMu serializes reloads and protects onReload.
	reloadMu syncutil.Mutex
	// onReload is called after each successful reload.
	onReload []func()

	cur atomic.Pointer[certSet]
}

// ErrCertManagement marks errors from loading the certificate, key, CA and
// rules files, for errors.Is.
var ErrCertManagement = errors.New("cannot load certificates")

// certSet is one loaded set of files. It is never changed after loading.
type certSet struct {
	// node is the node cert with its key, nil if the config has none.
	node *tls.Certificate
	// nodeCAs and clientCAs are the trust pools, nil if not configured.
	nodeCAs, clientCAs *x509.CertPool
	// nodeCACerts and clientCACerts are the certificates in the pools.
	nodeCACerts, clientCACerts []*x509.Certificate
	// client is the optional client cert with its key.
	client *tls.Certificate
	// rules are the identity rules, empty if not configured.
	rules *Rules
	// files holds the raw contents of every cert file, for ListCertificates.
	files []CertFile
}

// NewCertificateManager loads the files named by cfg. Fields of cfg may be
// empty; the methods that need a missing file return an error.
func NewCertificateManager(
	cfg certconfig.Config, tlsSettings TLSSettings,
) (*CertificateManager, error) {
	cm := &CertificateManager{
		cfg:         cfg,
		tlsSettings: tlsSettings,
		certMetrics: makeMetrics(),
	}
	s, err := loadCertSet(cfg)
	if err != nil {
		return nil, errors.Mark(err, ErrCertManagement)
	}
	cm.cur.Store(s)
	cm.updateMetrics(s)
	return cm, nil
}

// Metrics returns the metrics struct.
func (cm *CertificateManager) Metrics() Metrics {
	return cm.certMetrics
}

// Rules returns the current identity rules.
func (cm *CertificateManager) Rules() *Rules {
	return cm.cur.Load().rules
}

// OnReload registers f to run after each successful reload.
func (cm *CertificateManager) OnReload(f func()) {
	cm.reloadMu.Lock()
	defer cm.reloadMu.Unlock()
	cm.onReload = append(cm.onReload, f)
}

// Reload reads every file again. It swaps in the new set only if all of it
// loads; otherwise it keeps the old set, logs the error and counts it.
func (cm *CertificateManager) Reload(ctx context.Context) error {
	cm.reloadMu.Lock()
	defer cm.reloadMu.Unlock()
	s, err := loadCertSet(cm.cfg)
	if err != nil {
		cm.certMetrics.ReloadFailures.Inc(1)
		log.Ops.Errorf(ctx, "kept the old certificates and rules: %v", err)
		log.StructuredEvent(ctx, &eventpb.CertsReload{Success: false, ErrorMessage: err.Error()})
		return err
	}
	cm.cur.Store(s)
	cm.updateMetrics(s)
	if cache := cm.clientCertExpirationCache; cache != nil {
		cache.Clear()
	}
	log.StructuredEvent(ctx, &eventpb.CertsReload{Success: true})
	for _, f := range cm.onReload {
		f()
	}
	return nil
}

// RegisterExpirationCache registers a cache for client certificate expiration.
// It is called during server startup.
func (cm *CertificateManager) RegisterExpirationCache(cache *ClientCertExpirationCache) {
	cm.clientCertExpirationCache = cache
}

// MaybeUpsertClientExpiration updates or inserts the expiration time for the
// given client certificate. An update is contingent on whether the old
// expiration is after the new expiration.
func (cm *CertificateManager) MaybeUpsertClientExpiration(
	ctx context.Context, identity username.SQLUsername, expiration int64,
) {
	if cache := cm.clientCertExpirationCache; cache != nil {
		cache.MaybeUpsert(ctx,
			identity.Normalized(),
			expiration,
			cm.certMetrics.ClientExpiration,
		)
	}
}

func (cm *CertificateManager) updateMetrics(s *certSet) {
	earliest := func(certs []*x509.Certificate) int64 {
		var t time.Time
		for _, c := range certs {
			if t.IsZero() || c.NotAfter.Before(t) {
				t = c.NotAfter
			}
		}
		if t.IsZero() {
			return 0
		}
		return t.Unix()
	}
	cm.certMetrics.CAExpiration.Update(earliest(s.nodeCACerts))
	cm.certMetrics.ClientCAExpiration.Update(earliest(s.clientCACerts))
	var node int64
	if s.node != nil {
		node = s.node.Leaf.NotAfter.Unix()
	}
	cm.certMetrics.NodeExpiration.Update(node)
}

// PemUsage says what a certificate file is for.
type PemUsage uint32

const (
	_ PemUsage = iota
	// CAPem is the node CA file.
	CAPem
	// ClientCAPem is the client CA file.
	ClientCAPem
	// NodePem is the node cert file.
	NodePem
	// ClientPem is the client cert file.
	ClientPem
)

// CertFile is one loaded certificate file.
type CertFile struct {
	Usage    PemUsage
	Filename string
	Contents []byte
}

// ListCertificates returns the certificate files of the current set.
func (cm *CertificateManager) ListCertificates() []CertFile {
	return cm.cur.Load().files
}

// NodeCert returns the leaf of the node cert, or nil.
func (cm *CertificateManager) NodeCert() *x509.Certificate {
	if n := cm.cur.Load().node; n != nil {
		return n.Leaf
	}
	return nil
}

var skipPermissionChecks = runtime.GOOS == "windows" ||
	envutil.EnvOrDefaultBool("COCKROACH_SKIP_KEY_PERMISSION_CHECK", false)

const (
	// maxKeyPermissions is the maximum permissions for a key file. It
	// applies when the key file is not owned by root.
	maxKeyPermissions os.FileMode = 0700
	// maxGroupKeyPermissions is the maximum permissions for a key file owned
	// by root and by the group of this process.
	maxGroupKeyPermissions os.FileMode = 0740
)

func readFile(path string) ([]byte, error) {
	b, err := securityassets.GetLoader().ReadFile(path)
	if err != nil {
		return nil, errors.Wrapf(err, "reading %s", path)
	}
	return b, nil
}

// readKeyFile reads a key file after checking that only its owner can read
// it.
func readKeyFile(path string) ([]byte, error) {
	info, err := securityassets.GetLoader().Stat(path)
	if err != nil {
		return nil, errors.Wrapf(err, "could not stat key file %s", path)
	}
	if !info.Mode().IsRegular() {
		return nil, errors.Errorf("key file %s is not a regular file", path)
	}
	if !skipPermissionChecks {
		if err := checkFilePermissions(os.Getgid(), path, sysutil.GetFileACLInfo(info)); err != nil {
			return nil, err
		}
	}
	return readFile(path)
}

// loadKeyPair loads a cert file, leaf first, with its key.
func loadKeyPair(certPath, keyPath string) (*tls.Certificate, []byte, error) {
	certPEM, err := readFile(certPath)
	if err != nil {
		return nil, nil, err
	}
	keyPEM, err := readKeyFile(keyPath)
	if err != nil {
		return nil, nil, err
	}
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "loading %s with key %s", certPath, keyPath)
	}
	return &pair, certPEM, nil
}

func loadCAFile(path string) (*x509.CertPool, []*x509.Certificate, []byte, error) {
	b, err := readFile(path)
	if err != nil {
		return nil, nil, nil, err
	}
	certs, err := PEMContentsToX509(b)
	if err != nil {
		return nil, nil, nil, errors.Wrapf(err, "parsing CA file %s", path)
	}
	if len(certs) == 0 {
		return nil, nil, nil, errors.Errorf("no certificates in CA file %s", path)
	}
	pool := x509.NewCertPool()
	for _, c := range certs {
		pool.AddCert(c)
	}
	return pool, certs, b, nil
}

func hasExtKeyUsage(c *x509.Certificate, u x509.ExtKeyUsage) bool {
	for _, have := range c.ExtKeyUsage {
		if have == u || have == x509.ExtKeyUsageAny {
			return true
		}
	}
	return false
}

func loadCertSet(cfg certconfig.Config) (*certSet, error) {
	s := &certSet{rules: &Rules{}}
	var err error
	if cfg.NodeCA != "" {
		var b []byte
		if s.nodeCAs, s.nodeCACerts, b, err = loadCAFile(cfg.NodeCA); err != nil {
			return nil, err
		}
		s.files = append(s.files, CertFile{Usage: CAPem, Filename: cfg.NodeCA, Contents: b})
	}
	if cfg.ClientCA != "" {
		var b []byte
		if s.clientCAs, s.clientCACerts, b, err = loadCAFile(cfg.ClientCA); err != nil {
			return nil, err
		}
		s.files = append(s.files, CertFile{Usage: ClientCAPem, Filename: cfg.ClientCA, Contents: b})
	}
	if cfg.Cert != "" {
		var b []byte
		if s.node, b, err = loadKeyPair(cfg.Cert, cfg.Key); err != nil {
			return nil, err
		}
		leaf := s.node.Leaf
		if !hasExtKeyUsage(leaf, x509.ExtKeyUsageServerAuth) || !hasExtKeyUsage(leaf, x509.ExtKeyUsageClientAuth) {
			return nil, errors.Errorf("node cert %s needs both server and client extended key usage", cfg.Cert)
		}
		if s.nodeCAs != nil {
			if _, err := verifyChain(s.node.Leaf, s.node.Certificate, s.nodeCAs, x509.ExtKeyUsageServerAuth); err != nil {
				return nil, errors.Wrapf(err, "node cert %s does not verify against node CA %s", cfg.Cert, cfg.NodeCA)
			}
		}
		s.files = append(s.files, CertFile{Usage: NodePem, Filename: cfg.Cert, Contents: b})
	}
	if cfg.ClientCert != "" {
		var b []byte
		if s.client, b, err = loadKeyPair(cfg.ClientCert, cfg.ClientKey); err != nil {
			return nil, err
		}
		s.files = append(s.files, CertFile{Usage: ClientPem, Filename: cfg.ClientCert, Contents: b})
	}
	if cfg.Rules != "" {
		b, err := readFile(cfg.Rules)
		if err != nil {
			return nil, err
		}
		if s.rules, err = ParseRules(b); err != nil {
			return nil, errors.Wrapf(err, "parsing %s", cfg.Rules)
		}
	}
	return s, nil
}

// verifyChain verifies leaf, with the intermediates in rawChain[1:],
// against roots for the given usage.
func verifyChain(
	leaf *x509.Certificate, rawChain [][]byte, roots *x509.CertPool, usage x509.ExtKeyUsage,
) ([][]*x509.Certificate, error) {
	inter := x509.NewCertPool()
	for _, raw := range rawChain[1:] {
		c, err := x509.ParseCertificate(raw)
		if err != nil {
			return nil, err
		}
		inter.AddCert(c)
	}
	return leaf.Verify(x509.VerifyOptions{
		Roots:         roots,
		Intermediates: inter,
		KeyUsages:     []x509.ExtKeyUsage{usage},
		CurrentTime:   timeutil.Now(),
	})
}

// verifyPeer verifies a peer's chain against roots and runs the OCSP check.
func (cm *CertificateManager) verifyPeer(
	certs []*x509.Certificate, roots *x509.CertPool, usage x509.ExtKeyUsage,
) error {
	if len(certs) == 0 {
		return errors.New("peer sent no certificate")
	}
	if roots == nil {
		return errors.New("no CA file configured")
	}
	inter := x509.NewCertPool()
	for _, c := range certs[1:] {
		inter.AddCert(c)
	}
	chains, err := certs[0].Verify(x509.VerifyOptions{
		Roots:         roots,
		Intermediates: inter,
		KeyUsages:     []x509.ExtKeyUsage{usage},
		CurrentTime:   timeutil.Now(),
	})
	if err != nil {
		return err
	}
	return makeOCSPVerifier(cm.tlsSettings)(nil, chains)
}

// VerifyNode checks that certs, leaf first, is a node: it chains to the node
// CA with the given usage and matches a node rule.
func (cm *CertificateManager) VerifyNode(certs []*x509.Certificate, usage x509.ExtKeyUsage) error {
	s := cm.cur.Load()
	if err := cm.verifyPeer(certs, s.nodeCAs, usage); err != nil {
		return errors.Wrap(err, "verifying node certificate")
	}
	if !hasExtKeyUsage(certs[0], x509.ExtKeyUsageServerAuth) || !hasExtKeyUsage(certs[0], x509.ExtKeyUsageClientAuth) {
		return errors.Errorf("certificate %s needs both server and client extended key usage to be a node",
			DescribeCert(certs[0]))
	}
	if !s.rules.MatchesNode(certs[0]) {
		return errors.Errorf("certificate %s matches no node rule", DescribeCert(certs[0]))
	}
	return nil
}

// ClientRoles returns the roles that a SQL or HTTP client certificate maps
// to through the client rules, in rule order. It returns an error if the
// certificate does not chain to the client CA or matches no client rule.
func (cm *CertificateManager) ClientRoles(
	certs []*x509.Certificate,
) ([]username.SQLUsername, error) {
	s := cm.cur.Load()
	if err := cm.verifyPeer(certs, s.clientCAs, x509.ExtKeyUsageClientAuth); err != nil {
		return nil, errors.Wrap(err, "verifying client certificate")
	}
	roles := s.rules.ClientRoles(certs[0])
	if len(roles) == 0 {
		return nil, errors.Errorf("certificate %s matches no client rule", DescribeCert(certs[0]))
	}
	return roles, nil
}

// HTTPUser returns the user an HTTP client authenticates as. A node
// certificate is root. A client certificate is the role of the first client
// rule it matches.
func (cm *CertificateManager) HTTPUser(certs []*x509.Certificate) (username.SQLUsername, error) {
	if len(certs) == 0 {
		return username.SQLUsername{}, errors.New("no client certificate")
	}
	nodeErr := cm.VerifyNode(certs, x509.ExtKeyUsageClientAuth)
	if nodeErr == nil {
		return username.RootUserName(), nil
	}
	roles, err := cm.ClientRoles(certs)
	if err != nil {
		return username.SQLUsername{}, errors.CombineErrors(err, nodeErr)
	}
	return roles[0], nil
}

func (cm *CertificateManager) baseTLSConfig() *tls.Config {
	cfg := &tls.Config{
		CipherSuites: RecommendedCipherSuites(),
		MinVersion:   tls.VersionTLS12,
	}
	if cm.tlsSettings.oldCipherSuitesEnabled() {
		cfg.CipherSuites = append(cfg.CipherSuites, OldCipherSuites()...)
	}
	return cfg
}

func (cm *CertificateManager) nodeCertOrError() (*tls.Certificate, error) {
	if n := cm.cur.Load().node; n != nil {
		return n, nil
	}
	return nil, errors.New("the config file names no node cert")
}

// serverTLSConfig returns a server config that presents the node cert and
// asks for a client cert. verify checks the client's certificates; nil
// means the client cert is optional and checked later.
func (cm *CertificateManager) serverTLSConfig(
	pool func(*certSet) *x509.CertPool, verify func([]*x509.Certificate) error,
) (*tls.Config, error) {
	if _, err := cm.nodeCertOrError(); err != nil {
		return nil, err
	}
	cfg := cm.baseTLSConfig()
	// A resumed session skips the CA check, so a CA removed by a reload
	// would still be trusted. Tickets are off so every connection is
	// checked against the current set.
	cfg.SessionTicketsDisabled = true
	cfg.GetCertificate = func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
		return cm.nodeCertOrError()
	}
	base := cfg.Clone()
	cfg.GetConfigForClient = func(*tls.ClientHelloInfo) (*tls.Config, error) {
		c := base.Clone()
		// The pool only tells the client which CAs we accept; verify does
		// the check against the current set.
		c.ClientCAs = pool(cm.cur.Load())
		if verify == nil {
			c.ClientAuth = tls.RequestClientCert
			return c, nil
		}
		c.ClientAuth = tls.RequireAnyClientCert
		c.VerifyConnection = func(cs tls.ConnectionState) error {
			return verify(cs.PeerCertificates)
		}
		return c, nil
	}
	return cfg, nil
}

// RPCServerTLSConfig is the config of the RPC port. It admits only peers
// whose cert chains to the node CA and matches a node rule.
func (cm *CertificateManager) RPCServerTLSConfig() (*tls.Config, error) {
	return cm.serverTLSConfig(
		func(s *certSet) *x509.CertPool { return s.nodeCAs },
		func(certs []*x509.Certificate) error {
			return cm.VerifyNode(certs, x509.ExtKeyUsageClientAuth)
		})
}

// SQLServerTLSConfig is the config of the SQL port. It admits only clients
// whose cert chains to the client CA; pgwire then maps it to a role with
// ClientRoles.
func (cm *CertificateManager) SQLServerTLSConfig() (*tls.Config, error) {
	return cm.serverTLSConfig(
		func(s *certSet) *x509.CertPool { return s.clientCAs },
		func(certs []*x509.Certificate) error {
			s := cm.cur.Load()
			return cm.verifyPeer(certs, s.clientCAs, x509.ExtKeyUsageClientAuth)
		})
}

// HTTPServerTLSConfig is the config of the HTTP port. The client cert is
// optional here: /health and /_status/vars need none. The HTTP auth layer
// checks it with HTTPUser.
func (cm *CertificateManager) HTTPServerTLSConfig() (*tls.Config, error) {
	return cm.serverTLSConfig(
		func(s *certSet) *x509.CertPool {
			if s.nodeCAs == nil || s.clientCAs == nil {
				return nil
			}
			pool := s.nodeCAs.Clone()
			for _, c := range s.clientCACerts {
				pool.AddCert(c)
			}
			return pool
		},
		nil)
}

// NodeClientTLSConfig is the config a node, or an admin CLI command, uses
// to dial a node. It presents the node cert and admits the server only if
// its cert chains to the node CA and matches a node rule. The server's host
// name is not checked: the node rules say which certs are nodes.
func (cm *CertificateManager) NodeClientTLSConfig() (*tls.Config, error) {
	if _, err := cm.nodeCertOrError(); err != nil {
		return nil, err
	}
	cfg := cm.baseTLSConfig()
	cfg.GetClientCertificate = func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
		return cm.nodeCertOrError()
	}
	// The standard check would verify the host name against a fixed pool.
	// VerifyConnection checks the chain against the current node CA and the
	// node rules instead.
	cfg.InsecureSkipVerify = true
	cfg.VerifyConnection = func(cs tls.ConnectionState) error {
		return cm.VerifyNode(cs.PeerCertificates, x509.ExtKeyUsageServerAuth)
	}
	return cfg, nil
}
