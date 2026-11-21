// Copyright 2020 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package rpc

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/multitenant/tenantcapabilities"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/security"
	"github.com/cockroachdb/cockroach/pkg/security/certconfig"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/log/severity"
	"github.com/cockroachdb/errors"
)

type lazyHTTPClient struct {
	sync.Once
	httpClient http.Client
	err        error
}

type lazyCertificateManager struct {
	sync.Once
	cm  *security.CertificateManager
	err error
}

func wrapError(err error) error {
	if err == nil {
		return nil
	}
	return errors.Wrap(err, "problem using security settings")
}

// SecurityContextOptions contains the subset of base.Config
// useful to define a SecurityContext.
type SecurityContextOptions struct {
	// Certs names the cert, key, CA and rules files.
	Certs    certconfig.Config
	Insecure bool

	// DisableTLSForHTTP is only used by GetHTTPServerTLSConfig().
	DisableTLSForHTTP bool
	// AdvertiseAddrH is only used by CheckCertificateAddrs().
	*base.AdvertiseAddrH
	// SQLAdvertiseAddrH is only used by CheckCertificateAddrs().
	*base.SQLAdvertiseAddrH
}

// SecurityContext is a wrapper providing transport security helpers such as
// the certificate manager.
type SecurityContext struct {
	security.TLSSettings
	config                 SecurityContextOptions
	tenID                  roachpb.TenantID
	capabilitiesAuthorizer tenantcapabilities.Authorizer
	lazy                   struct {
		// The certificate manager. Must be accessed through GetCertificateManager.
		certificateManager lazyCertificateManager
		// httpClient uses the client TLS config. It is initialized lazily.
		httpClient lazyHTTPClient
	}
}

// NewSecurityContext instantiates a SecurityContext.
func NewSecurityContext(
	cfg SecurityContextOptions,
	tlsSettings security.TLSSettings,
	tenID roachpb.TenantID,
	capabilitiesAuthorizer tenantcapabilities.Authorizer,
) *SecurityContext {
	if tenID.ToUint64() == 0 {
		panic(errors.AssertionFailedf("programming error: tenant ID not defined"))
	}
	return &SecurityContext{
		TLSSettings:            tlsSettings,
		config:                 cfg,
		tenID:                  tenID,
		capabilitiesAuthorizer: capabilitiesAuthorizer,
	}
}

// GetCertificateManager returns the certificate manager, loading the files
// on the first call.
func (ctx *SecurityContext) GetCertificateManager() (*security.CertificateManager, error) {
	ctx.lazy.certificateManager.Do(func() {
		ctx.lazy.certificateManager.cm, ctx.lazy.certificateManager.err =
			security.NewCertificateManager(ctx.config.Certs, ctx)
	})
	return ctx.lazy.certificateManager.cm, ctx.lazy.certificateManager.err
}

// tlsConfig returns nil in insecure mode, and otherwise the config that get
// builds from the certificate manager.
func (ctx *SecurityContext) tlsConfig(
	get func(*security.CertificateManager) (*tls.Config, error),
) (*tls.Config, error) {
	if ctx.config.Insecure {
		return nil, nil
	}
	cm, err := ctx.GetCertificateManager()
	if err != nil {
		return nil, wrapError(err)
	}
	tlsCfg, err := get(cm)
	return tlsCfg, wrapError(err)
}

// GetServerTLSConfig returns the TLS config of the RPC port, or nil in
// insecure mode. It admits only nodes.
func (ctx *SecurityContext) GetServerTLSConfig() (*tls.Config, error) {
	return ctx.tlsConfig((*security.CertificateManager).RPCServerTLSConfig)
}

// GetSQLServerTLSConfig returns the TLS config of the SQL port, or nil in
// insecure mode. It admits only clients of the client CA.
func (ctx *SecurityContext) GetSQLServerTLSConfig() (*tls.Config, error) {
	return ctx.tlsConfig((*security.CertificateManager).SQLServerTLSConfig)
}

// GetHTTPServerTLSConfig returns the TLS config of the HTTP port, or nil in
// insecure mode or when TLS is disabled for HTTP.
func (ctx *SecurityContext) GetHTTPServerTLSConfig() (*tls.Config, error) {
	if ctx.config.DisableTLSForHTTP {
		return nil, nil
	}
	return ctx.tlsConfig((*security.CertificateManager).HTTPServerTLSConfig)
}

// GetHTTPClient returns the http client, initializing it if needed. It
// connects to other nodes with the node cert.
func (ctx *SecurityContext) GetHTTPClient() (http.Client, error) {
	ctx.lazy.httpClient.Do(func() {
		ctx.lazy.httpClient.httpClient.Timeout = 10 * time.Second
		var transport http.Transport
		ctx.lazy.httpClient.httpClient.Transport = &transport
		if !ctx.config.DisableTLSForHTTP {
			transport.TLSClientConfig, ctx.lazy.httpClient.err = ctx.tlsConfig(
				(*security.CertificateManager).NodeClientTLSConfig)
		}
	})

	return ctx.lazy.httpClient.httpClient, ctx.lazy.httpClient.err
}

// CheckCertificateAddrs warns if the node cert does not name the advertised
// addresses. SQL and HTTP clients check the host name with
// sslmode=verify-full; nodes do not, they check the node rules.
// This must be called after ValidateAddrs() and after the certificate
// manager was initialized.
func (ctx *SecurityContext) CheckCertificateAddrs(cctx context.Context) {
	if ctx.config.Insecure {
		return
	}
	cm, err := ctx.GetCertificateManager()
	if err != nil {
		return
	}
	cert := cm.NodeCert()
	if cert == nil {
		return
	}
	addrInfo := certAddrs(cert)
	log.Ops.Infof(cctx, "node certificate addresses: %s", addrInfo)

	host, _, err := net.SplitHostPort(ctx.config.SQLAdvertiseAddr)
	if err != nil {
		log.Ops.Errorf(cctx, "invalid SQL advertise address %q: %v", ctx.config.SQLAdvertiseAddr, err)
		return
	}
	if err := cert.VerifyHostname(host); err != nil {
		log.Ops.Shoutf(cctx, severity.WARNING,
			"advertise SQL address %q not in node certificate (%s)\n"+
				"SQL clients that use sslmode=verify-full will fail to connect.",
			host, addrInfo)
	}
}

// HTTPRequestScheme returns "http" or "https" based on the value of
// Insecure and DisableTLSForHTTP.
func (ctx *SecurityContext) HTTPRequestScheme() string {
	if ctx.config.Insecure || ctx.config.DisableTLSForHTTP {
		return "http"
	}
	return "https"
}

// certAddrs formats the list of addresses included in a certificate for
// printing in an error message.
func certAddrs(cert *x509.Certificate) string {
	// If an IP address was specified as listen/adv address, the
	// hostname validation will only use the IPAddresses field. So this
	// needs to be printed in all cases.
	addrs := make([]string, len(cert.IPAddresses))
	for i, ip := range cert.IPAddresses {
		addrs[i] = ip.String()
	}
	// For names, the hostname validation will use DNSNames if
	// the Subject Alt Name is present in the cert, otherwise
	// it will use the common name. We can't parse the
	// extensions here so we print both.
	return fmt.Sprintf("IP=%s; DNS=%s; CN=%s",
		strings.Join(addrs, ","),
		strings.Join(cert.DNSNames, ","),
		cert.Subject.CommonName)
}
