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

import "github.com/cockroachdb/cockroach/pkg/server/pgurl"

// CLIFlagInterfaceForClientURL is an interface to the state of the
// variables attached to discrete flags (e.g. --host, --insecure)
// that can be passed before or after --url.
//
// It is used by AnalyzeClientURL below to peek into the state
// of flags set before --url and prepare state for subsequent
// flags passed after --url.
//
// NB: We use an interface here instead of directly depending on
// packages pflag and cliflags, so as to simplify the build
// dependencies and increase readability.
type CLIFlagInterfaceForClientURL interface {
	// StrictTLS returns true when the URL must either be insecure or
	// fully verify the server.
	//
	// This is set for all non-SQL client commands. They take their
	// certificates from the --config file, not from the URL.
	StrictTLS() bool

	// NewStrictTLSConfigurationError will be called when a URL contains
	// parameters that conflict with the requirements of StrictTLS().
	NewStrictTLSConfigurationError() error

	// SetUser is called when the URL contains a username.
	SetUser(string)

	// SetHost is called when the URL contains a server name / address.
	SetHost(string)

	// SetPort is called when the URL contains a server port name / number.
	SetPort(string)

	// SetDatabase is called when the URL contains a database name.
	SetDatabase(string)

	// InsecureFlag should return whether --insecure was specified and
	// its value.
	InsecureFlag() (bool, bool)

	// SetInsecure is called when the URL is found to imply --insecure.
	// TODO(knz): This behavior should be removed.
	// See https://github.com/cockroachdb/cockroach/issues/54007.
	SetInsecure(bool)
}

// AnalyzeClientURL is a helper that processes a connection URL passed on the
// command line, integrates any defaults set by earlier command-line arguments,
// then calls callbacks depending on which details were found in the new URL.
// Its focus is on TLS authentication options.
//
// flags.StrictTLS(), when true, restricts the supported SSL modes to
// "disable" and "verify-full".
func AnalyzeClientURL(newURL string, flags CLIFlagInterfaceForClientURL) (*pgurl.URL, error) {
	parsedURL, err := pgurl.Parse(newURL)
	if err != nil {
		return nil, err
	}

	if user := parsedURL.GetUsername(); user != "" {
		flags.SetUser(user)
	}

	// If some host/port information is available, forward it to
	// --host / --port.
	net, host, port := parsedURL.GetNetworking()
	if host != "" {
		flags.SetHost(host)
	}
	if port != "" {
		flags.SetPort(port)
	}

	// If a database is specified, and the command supports databases,
	// forward it to --database.
	if db := parsedURL.GetDatabase(); db != "" {
		flags.SetDatabase(db)
	}

	tlsUsed, tlsMode, caCertPath := parsedURL.GetTLSOptions()

	if tlsUsed && tlsMode == pgurl.TLSUnspecified && net == pgurl.ProtoTCP {
		// The sslmode argument was not specified and we are using TCP.
		// We may want to inject a default value in that case.
		// (We don't inject a transport if using unix sockets.)
		//
		// Is there a value to go by from a previous --insecure flag? If
		// so, use that.
		if insecureSpecified, insecureValue := flags.InsecureFlag(); insecureSpecified {
			var tp pgurl.TransportOption
			if insecureValue {
				tp = pgurl.TransportNone()
				tlsUsed = false
			} else {
				tlsMode = pgurl.TLSVerifyFull
				tp = pgurl.TransportTLS(tlsMode, caCertPath)
			}
			parsedURL.WithTransport(tp)
		} else {
			// No --insecure specified. We default to maximum security.
			tlsMode = pgurl.TLSVerifyFull
			parsedURL.WithTransport(pgurl.TransportTLS(tlsMode, caCertPath))
		}
	}

	if !tlsUsed {
		if flags.StrictTLS() {
			// For "strict" mode (RPC client commands) we don't support non-TLS
			// yet. See https://github.com/cockroachdb/cockroach/issues/54007
			// Instead, we see a request for no TLS to imply insecure mode.
			flags.SetInsecure(true)
		}
	} else {
		if flags.StrictTLS() {
			switch tlsMode {
			case pgurl.TLSVerifyFull:
				// This is valid.
			default:
				return nil, flags.NewStrictTLSConfigurationError()
			}
		}
		flags.SetInsecure(false)
	}

	// Check that the URL so far is valid.
	err = parsedURL.Validate()
	return parsedURL, err
}
