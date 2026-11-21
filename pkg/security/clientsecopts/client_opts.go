// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package clientsecopts

import (
	"net/url"

	"github.com/cockroachdb/cockroach/pkg/security/certconfig"
	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/cockroach/pkg/server/pgurl"
	"github.com/cockroachdb/cockroach/pkg/util/netutil/addr"
)

// ClientSecurityOptions defines the configurable connection parameters
// used as input to compute the security parameters of connection URLs.
type ClientSecurityOptions struct {
	// Insecure corresponds to the --insecure flag.
	Insecure bool

	// Certs comes from the --config file. The client verifies servers with
	// its node CA and presents its client cert.
	Certs certconfig.Config
}

// loadSecurityOptions extends a url.Values with SSL settings suitable for
// the given server config.
func loadSecurityOptions(
	opts ClientSecurityOptions, u *pgurl.URL, user username.SQLUsername,
) error {
	u.WithUsername(user.Normalized())
	if opts.Insecure {
		u.WithInsecure()
	} else if net, _, _ := u.GetNetworking(); net == pgurl.ProtoTCP {
		tlsUsed, tlsMode, caCertPath := u.GetTLSOptions()
		if !tlsUsed {
			// TLS explicitly disabled by client. Nothing to do here.
			return nil
		}
		// Default is to verify the server's identity.
		if tlsMode == pgurl.TLSUnspecified {
			tlsMode = pgurl.TLSVerifyFull
		}
		// Servers present node certs, so the node CA verifies them. Without
		// one, the Go TLS code falls back to the OS trust store.
		if caCertPath == "" && (tlsMode == pgurl.TLSVerifyFull || tlsMode == pgurl.TLSVerifyCA) {
			caCertPath = opts.Certs.NodeCA
		}
		u.WithTransport(pgurl.TransportTLS(tlsMode, caCertPath))

		// A client cert named in the URL wins over the config's.
		if hasCert, _, _ := u.GetAuthnCert(); !hasCert && opts.Certs.ClientCert != "" {
			u.WithAuthn(pgurl.AuthnClientCert(opts.Certs.ClientCert, opts.Certs.ClientKey))
		}
	}
	return nil
}

// ServerParameters is used to pass a copy of the parameters computed
// when a SQL server starts up, to the input of PGURL(), to create
// a connection URL that clients can connect to.
type ServerParameters struct {
	// ServerAddr is the configured server address to use if the URL does not contain one.
	ServerAddr string

	// DefaultPort is the port number to use if ServerAddr does not contain one.
	DefaultPort string

	// DefaultDatabase is the default database name to use if the connection URL
	// does not contain one.
	DefaultDatabase string
}

// MakeURLForServer constructs a URL for the postgres endpoint, given a server
// config.
func MakeURLForServer(
	copts ClientSecurityOptions, sparams ServerParameters, user *url.Userinfo,
) (*pgurl.URL, error) {
	host, port, _ := addr.SplitHostPort(sparams.ServerAddr, sparams.DefaultPort)
	u := pgurl.New().
		WithNet(pgurl.NetTCP(host, port)).
		WithDatabase(sparams.DefaultDatabase)

	username, _ := username.MakeSQLUsernameFromUserInput(user.Username(), username.PurposeValidation)
	if err := loadSecurityOptions(copts, u, username); err != nil {
		return nil, err
	}
	return u, nil
}

// ClientOptions represents configurable options from the command line.
type ClientOptions struct {
	ClientSecurityOptions

	// ExplicitURL is an explicit URL specified on the command line
	// via --url, and enhanced via clientconnurl.UpdateURL().
	// This may contain additional parameters (e.g. application_name)
	// which cannot be specified by discrete flags.
	ExplicitURL *pgurl.URL

	// User is the requested username, as specified via --user or --url.
	//
	// When calling UpdateURL() to generate ExplicitURL from --url, the
	// function should be passed an update callback that updates this
	// field if the URL contains a username part.
	User string

	// Database is the requested database name, as specified
	// via --database or --url.
	//
	// When calling UpdateURL() to generate ExplicitURL from --url, the
	// function should be passed an update callback that updates this
	// field if the URL contains a database name.
	Database string

	// ServerHost is the requested server host name or address, as
	// specified via --host or --url.
	//
	// When calling UpdateURL() to generate ExplicitURL from --url, the
	// function should be passed an update callback that updates this
	// field if the URL contains a hostname part.
	ServerHost string

	// ServerPort is the requested server port name/number, as
	// specified via --port or --url.
	//
	// When calling UpdateURL() to generate ExplicitURL from --url, the
	// function should be passed an update callback that updates this
	// field if the URL contains a port number/name.
	ServerPort string
}

// MakeClientConnURL constructs a connection URL from the given input options.
func MakeClientConnURL(copts ClientOptions) (*pgurl.URL, error) {
	purl := copts.ExplicitURL
	if purl == nil {
		// New URL. Start from scratch.
		purl = pgurl.New() // defaults filled in below.
	}

	// Fill in any defaults from any command-line arguments if there was
	// no --url flag, or if they were specified *after* the --url flag.
	//
	// Note: the username is filled in by loadSecurityOptions() below.
	// If there was any password while parsing a --url flag,
	// it will be pre-populated via cliCtx.sqlConnURL above.
	purl.WithDatabase(copts.Database)
	if _, host, port := purl.GetNetworking(); host != copts.ServerHost || port != copts.ServerPort {
		purl.WithNet(pgurl.NetTCP(copts.ServerHost, copts.ServerPort))
	}

	// Check the structure of the username.
	userName, err := username.MakeSQLUsernameFromUserInput(copts.User, username.PurposeValidation)
	if err != nil {
		return nil, err
	}
	if userName.Undefined() {
		userName = username.RootUserName()
	}

	if err := loadSecurityOptions(copts.ClientSecurityOptions, purl, userName); err != nil {
		return nil, err
	}

	// The construct above should have produced a valid URL already;
	// however a post-assertion doesn't hurt.
	if err := purl.Validate(); err != nil {
		return nil, err
	}

	return purl, nil
}

// UsesConfigClientCert reports whether a connection made with these options
// presents the client cert of the --config file: it uses TLS and its URL
// names no client cert of its own.
func UsesConfigClientCert(insecure bool, u *pgurl.URL) bool {
	if insecure {
		return false
	}
	if u == nil {
		return true
	}
	if tlsUsed, _, _ := u.GetTLSOptions(); !tlsUsed {
		return false
	}
	hasCert, _, _ := u.GetAuthnCert()
	return !hasCert
}
