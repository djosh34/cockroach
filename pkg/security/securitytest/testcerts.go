// Copyright 2021 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package securitytest

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/cockroachdb/cockroach/pkg/security/certnames"
	"github.com/cockroachdb/cockroach/pkg/security/securityassets"
)

// EmbeddedTenantIDs lists the tenants we embed certs for.
// See 'test_certs/regenerate.sh'.
func EmbeddedTenantIDs() []uint64 { return []uint64{10, 11, 20, 2} }

// CreateTestCerts populates the test certificates in the given directory.
func CreateTestCerts(certsDir string) (cleanup func() error) {
	// Copy these assets to disk from embedded strings, so this test can
	// run from a standalone binary.
	// Disable embedded certs, or the security library will try to load
	// our real files as embedded assets.
	securityassets.ResetLoader()

	assets := []string{
		filepath.Join(certnames.EmbeddedCertsDir, certnames.EmbeddedCACert),
		filepath.Join(certnames.EmbeddedCertsDir, certnames.EmbeddedCAKey),
		filepath.Join(certnames.EmbeddedCertsDir, certnames.EmbeddedNodeCert),
		filepath.Join(certnames.EmbeddedCertsDir, certnames.EmbeddedNodeKey),
		filepath.Join(certnames.EmbeddedCertsDir, certnames.EmbeddedRootCert),
		filepath.Join(certnames.EmbeddedCertsDir, certnames.EmbeddedRootKey),
		filepath.Join(certnames.EmbeddedCertsDir, certnames.EmbeddedTenantCACert),
	}
	for _, tenantID := range EmbeddedTenantIDs() {
		st := strconv.Itoa(int(tenantID))
		assets = append(assets,
			filepath.Join(certnames.EmbeddedCertsDir, certnames.TenantCertFilename(st)),
			filepath.Join(certnames.EmbeddedCertsDir, certnames.TenantKeyFilename(st)),
		)
	}

	for _, a := range assets {
		_, err := RestrictedCopy(a, certsDir, filepath.Base(a))
		if err != nil {
			panic(err)
		}
	}

	return func() error {
		securityassets.SetLoader(EmbeddedAssets)
		return os.RemoveAll(certsDir)
	}
}
