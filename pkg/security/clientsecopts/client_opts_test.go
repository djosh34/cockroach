// Copyright 2026 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package clientsecopts_test

import (
	"testing"

	"github.com/cockroachdb/cockroach/pkg/security/certconfig"
	"github.com/cockroachdb/cockroach/pkg/security/clientsecopts"
	"github.com/cockroachdb/cockroach/pkg/server/pgurl"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/stretchr/testify/require"
)

func TestUsesConfigClientCert(t *testing.T) {
	defer leaktest.AfterTest(t)()
	certs := certconfig.Config{NodeCA: "ca.crt", ClientCert: "client.crt", ClientKey: "client.key"}
	uses := func(insecure bool, url, host, port string) bool {
		t.Helper()
		copts := clientsecopts.ClientOptions{
			ClientSecurityOptions: clientsecopts.ClientSecurityOptions{Insecure: insecure, Certs: certs},
			ServerHost:            host,
			ServerPort:            port,
		}
		if url != "" {
			u, err := pgurl.Parse(url)
			require.NoError(t, err)
			copts.ExplicitURL = u
			_, copts.ServerHost, copts.ServerPort = u.GetNetworking()
			if host != "" {
				copts.ServerHost, copts.ServerPort = host, port
			}
		}
		ok, err := clientsecopts.UsesConfigClientCert(copts)
		require.NoError(t, err)
		return ok
	}
	require.True(t, uses(false, "", "h", "26257"))
	require.False(t, uses(true, "", "h", "26257"))
	require.True(t, uses(false, "postgresql://h:26257?sslmode=verify-full", "", ""))
	require.False(t, uses(false, "postgresql://h:26257?sslmode=disable", "", ""))
	require.False(t, uses(false, "postgresql://h:26257?sslmode=verify-full&sslcert=c.crt&sslkey=c.key", "", ""))
	// Even when the URL names the config's own cert file.
	require.False(t, uses(false, "postgresql://h:26257?sslmode=verify-full&sslcert=client.crt&sslkey=other.key", "", ""))
	require.False(t, uses(false, "postgresql://root@/defaultdb?host=/tmp&port=26257", "", ""))
	// --host after a socket URL makes it TCP, with the config cert.
	require.True(t, uses(false, "postgresql://root@/defaultdb?host=/tmp&port=26257", "h", "26257"))
}
