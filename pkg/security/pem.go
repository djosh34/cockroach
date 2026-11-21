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
	"bytes"
	"crypto/x509"
	"encoding/pem"

	"github.com/cockroachdb/errors"
)

// PEMToCertificates parses multiple certificate PEM blocks and returns them.
// Each block must be a certificate.
// It is allowed to have zero certificates.
func PEMToCertificates(contents []byte) ([]*pem.Block, error) {
	certs := make([]*pem.Block, 0)
	for {
		var block *pem.Block
		block, contents = pem.Decode(contents)
		if block == nil {
			// Text that is not PEM is allowed, as in openssl's output, but
			// a broken block is not: it may be a half-written file.
			if bytes.Contains(contents, []byte("-----BEGIN")) {
				return nil, errors.Errorf("block #%d is not a complete PEM block", len(certs))
			}
			break
		}
		if block.Type != "CERTIFICATE" {
			return nil, errors.Errorf("block #%d is of type %s, not CERTIFICATE", len(certs), block.Type)
		}

		certs = append(certs, block)
	}

	return certs, nil
}

// PEMContentsToX509 takes raw pem-encoded contents and attempts to parse into
// x509.Certificate objects.
func PEMContentsToX509(contents []byte) ([]*x509.Certificate, error) {
	derCerts, err := PEMToCertificates(contents)
	if err != nil {
		return nil, err
	}

	certs := make([]*x509.Certificate, len(derCerts))
	for i, c := range derCerts {
		x509Cert, err := x509.ParseCertificate(c.Bytes)
		if err != nil {
			return nil, err
		}

		certs[i] = x509Cert
	}

	return certs, nil
}
