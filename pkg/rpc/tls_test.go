// Copyright 2015 The Cockroach Authors.
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
	"testing"

	"github.com/cockroachdb/cockroach/pkg/security/certconfig"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/testutils"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/stop"
)

// TestSSLSettings checks the TLS configs of an RPC context: none in
// insecure mode, and an error when a configured file is missing.
func TestSSLSettings(t *testing.T) {
	defer leaktest.AfterTest(t)()

	missing := certconfig.Embedded()
	missing.Cert = "i-do-not-exist.crt"

	testCases := []struct {
		insecure      bool
		certs         certconfig.Config
		requestScheme string
		configErr     string
	}{
		{true, certconfig.Config{}, "http", ""},
		{false, certconfig.Embedded(), "https", ""},
		{false, missing, "https", "i-do-not-exist.crt"},
	}

	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			opts := DefaultContextOptions()
			opts.Insecure = tc.insecure
			opts.Certs = tc.certs
			ctx := context.Background()
			stopper := stop.NewStopper()
			defer stopper.Stop(ctx)
			opts.Stopper = stopper
			opts.Settings = cluster.MakeTestingClusterSettings()
			rpcContext := NewContext(ctx, opts)

			if expected, actual := tc.requestScheme, rpcContext.SecurityContext.HTTPRequestScheme(); expected != actual {
				t.Fatalf("expected HTTPRequestScheme=%s, got: %s", expected, actual)
			}
			for _, get := range []func() (*tls.Config, error){
				rpcContext.GetClientTLSConfig,
				rpcContext.GetServerTLSConfig,
				rpcContext.GetSQLServerTLSConfig,
				rpcContext.GetHTTPServerTLSConfig,
			} {
				tlsConfig, err := get()
				if !testutils.IsError(err, tc.configErr) {
					t.Fatalf("expected err=%s, got err=%v", tc.configErr, err)
				}
				if err != nil {
					continue
				}
				if (tlsConfig == nil) != tc.insecure {
					t.Fatalf("expected nil config=%t, got: %+v", tc.insecure, tlsConfig)
				}
			}
		})
	}
}
