// Copyright 2016 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package sqlutils

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/security/certconfig"
	"github.com/cockroachdb/cockroach/pkg/security/securityassets"
	"github.com/cockroachdb/cockroach/pkg/util/fileutil"
)

// PGUrl is like PGUrlE, but uses t.Fatal to handle errors.
func PGUrl(t testing.TB, servingAddr, prefix string, user *url.Userinfo) (url.URL, func()) {
	return PGUrlWithOptionalClientCerts(t, servingAddr, prefix, user, true, "" /* withCerts */)
}

// PGUrlE returns a postgres connection url which connects to this server with the given user, and a
// cleanup function which must be called after all connections created using the connection url have
// been closed.
//
// In order to connect securely using postgres, this method will create temporary on-disk copies of
// the test certificates that the securitytest package serves from memory. The certificates will be
// created in a new temporary directory. The returned cleanup function will delete this temporary
// directory.
// Note that two calls to this function for the same `user` will generate different
// copies of the certificates, so the cleanup function must always be called.
//
// Args:
//
//	prefix: A prefix to be prepended to the temp file names generated, for debugging.
func PGUrlE(servingAddr, prefix string, user *url.Userinfo) (url.URL, func(), error) {
	return PGUrlWithOptionalClientCertsE(servingAddr, prefix, user, true, "" /* withCerts */)
}

// PGUrlWithOptionalClientCerts is like PGUrlWithOptionalClientCertsE, but uses t.Fatal to handle
// errors.
func PGUrlWithOptionalClientCerts(
	t testing.TB,
	servingAddr, prefix string,
	user *url.Userinfo,
	withClientCerts bool,
	certName string,
) (url.URL, func()) {
	u, f, err := PGUrlWithOptionalClientCertsE(servingAddr, prefix, user, withClientCerts, certName)
	if err != nil {
		t.Fatal(err)
	}
	return u, f
}

// PGUrlWithOptionalClientCertsE is like PGUrlE but the caller can
// customize whether the client certificates are loaded on-disk and in the URL.
// certName is name of the certificate to use. If empty, the username is used.
func PGUrlWithOptionalClientCertsE(
	servingAddr, prefix string, user *url.Userinfo, withClientCerts bool, certName string,
) (url.URL, func(), error) {
	host, port, err := net.SplitHostPort(servingAddr)
	if err != nil {
		return url.URL{}, func() {}, err
	}

	// TODO(benesch): Audit usage of prefix and replace the following line with
	// `testutils.TempDir(t)` if prefix can always be `t.Name()`.
	tempDir, err := os.MkdirTemp("", fileutil.EscapeFilename(prefix))
	if err != nil {
		return url.URL{}, func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(tempDir) }
	// copyCert copies a test file, which the securitytest package serves from
	// memory, to disk for the SQL driver.
	copyCert := func(name, dst string) (string, error) {
		b, err := securityassets.GetLoader().ReadFile(filepath.Join(certconfig.EmbeddedDir, name))
		if err != nil {
			return "", err
		}
		path := filepath.Join(tempDir, dst)
		return path, os.WriteFile(path, b, 0600)
	}

	options := url.Values{}
	// The client verifies the server's node cert against the node CA.
	caPath, err := copyCert("ca.crt", "ca")
	if err != nil {
		cleanup()
		return url.URL{}, func() {}, err
	}
	options.Add("sslrootcert", caPath)

	if withClientCerts {
		if certName == "" {
			certName = user.Username()
		}
		certPath, err := copyCert(fmt.Sprintf("client.%s.crt", certName), "cert")
		if err != nil {
			cleanup()
			return url.URL{}, func() {}, err
		}
		keyPath, err := copyCert(fmt.Sprintf("client.%s.key", certName), "key")
		if err != nil {
			cleanup()
			return url.URL{}, func() {}, err
		}
		options.Add("sslcert", certPath)
		options.Add("sslkey", keyPath)
		options.Add("sslmode", "verify-full")
	} else {
		options.Add("sslmode", "verify-ca")
	}

	return url.URL{
		Scheme:   "postgres",
		User:     user,
		Host:     net.JoinHostPort(host, port),
		RawQuery: options.Encode(),
	}, cleanup, nil
}
