// Copyright 2026 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

// Package securitytest makes test certificates with crypto/x509. Every key,
// serial number and validity time comes from a seed, so the same seed gives
// byte-identical certificates.
//
// This package is for tests only. CI fails if the patchroaddb binary links
// it.
package securitytest

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/pem"
	"math/big"
	"net"
	"net/url"
	"time"
)

// SPIFFEDomain is the SPIFFE trust domain of the test certificates.
const SPIFFEDomain = "patchroaddb.test"

// SPIFFEID returns the SPIFFE ID spiffe://patchroaddb.test/<path>.
func SPIFFEID(path string) string {
	return "spiffe://" + SPIFFEDomain + "/" + path
}

// Validity of every certificate made here.
var (
	notBefore = time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	notAfter  = time.Date(2099, 12, 31, 0, 0, 0, 0, time.UTC)
)

// Generator makes certificates from a seed.
type Generator struct {
	seed int64
	// fail reports an error. A method that fails returns empty values.
	fail func(error)
}

// NewGenerator returns a Generator for the seed. It calls fail on an error,
// such as a SPIFFE ID that does not parse; a test passes a function that
// calls t.Fatal.
func NewGenerator(seed int64, fail func(error)) *Generator {
	return &Generator{seed: seed, fail: fail}
}

// derive returns 32 bytes that depend only on the seed and the label.
func (g *Generator) derive(label string) []byte {
	h := sha256.New()
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(g.seed))
	h.Write(b[:])
	h.Write([]byte(label))
	return h.Sum(nil)
}

func (g *Generator) key(label string) ed25519.PrivateKey {
	return ed25519.NewKeyFromSeed(g.derive("key/" + label))
}

func (g *Generator) serial(label string) *big.Int {
	b := g.derive("serial/" + label)[:16]
	b[0] &= 0x7f // positive
	b[0] |= 0x40 // nonzero
	return new(big.Int).SetBytes(b)
}

// Identity is what a leaf certificate says about its holder. Empty fields
// are left out of the certificate.
type Identity struct {
	CN     string
	SPIFFE string
}

// CA is a certificate authority. If making it failed, Cert is nil and it
// makes no certificates.
type CA struct {
	g     *Generator
	label string
	Cert  *x509.Certificate
	// CertPEM is this CA's certificate.
	CertPEM []byte
	// chainPEM is what a leaf signed by this CA appends after itself: this
	// CA and its parents, without the root.
	chainPEM []byte
	key      ed25519.PrivateKey
}

// RootCA makes a self-signed root CA.
func (g *Generator) RootCA(cn string) *CA {
	label := "ca/" + cn
	key := g.key(label)
	tmpl := &x509.Certificate{
		SerialNumber:          g.serial(label),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	ca := &CA{g: g, label: label, key: key}
	ca.Cert, ca.CertPEM = g.create(tmpl, tmpl, key.Public(), key)
	return ca
}

// IntermediateCA makes a CA signed by ca. If usage is given, the CA may
// only sign certs for those extended key usages.
func (ca *CA) IntermediateCA(cn string, usage ...x509.ExtKeyUsage) *CA {
	label := ca.label + "/ca/" + cn
	if ca.Cert == nil {
		// Making ca failed and was reported.
		return &CA{g: ca.g, label: label}
	}
	key := ca.g.key(label)
	tmpl := &x509.Certificate{
		SerialNumber:          ca.g.serial(label),
		Subject:               pkix.Name{CommonName: cn},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		ExtKeyUsage:           usage,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	sub := &CA{g: ca.g, label: label, key: key}
	sub.Cert, sub.CertPEM = ca.g.create(tmpl, ca.Cert, key.Public(), ca.key)
	sub.chainPEM = append(append([]byte(nil), sub.CertPEM...), ca.chainPEM...)
	return sub
}

// Pair is a leaf certificate with its key. CertPEM holds the leaf, then the
// intermediates up to the root.
type Pair struct {
	CertPEM []byte
	KeyPEM  []byte
}

// NodeCert makes a cert for a node: server and client usage, and hosts as
// DNS names or IP addresses.
func (ca *CA) NodeCert(id Identity, hosts ...string) Pair {
	return ca.leaf("node", id, hosts,
		[]x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth})
}

// ClientCert makes a cert with client usage only.
func (ca *CA) ClientCert(id Identity) Pair {
	return ca.leaf("client", id, nil, []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth})
}

// CertWithUsage makes a cert with the given extended key usages, none if
// usage is empty.
func (ca *CA) CertWithUsage(id Identity, usage ...x509.ExtKeyUsage) Pair {
	return ca.leaf("usage", id, nil, usage)
}

func (ca *CA) leaf(kind string, id Identity, hosts []string, usage []x509.ExtKeyUsage) Pair {
	if ca.Cert == nil {
		// Making ca failed and was reported.
		return Pair{}
	}
	label := ca.label + "/" + kind + "/" + id.CN + "/" + id.SPIFFE
	for _, h := range hosts {
		label += "/" + h
	}
	key := ca.g.key(label)
	tmpl := &x509.Certificate{
		SerialNumber: ca.g.serial(label),
		Subject:      pkix.Name{CommonName: id.CN},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  usage,
	}
	if id.SPIFFE != "" {
		u, err := url.Parse(id.SPIFFE)
		if err != nil {
			ca.g.fail(err)
			return Pair{}
		}
		tmpl.URIs = []*url.URL{u}
	}
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, h)
		}
	}
	_, certPEM := ca.g.create(tmpl, ca.Cert, key.Public(), ca.key)
	if certPEM == nil {
		return Pair{}
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		ca.g.fail(err)
		return Pair{}
	}
	return Pair{
		CertPEM: append(certPEM, ca.chainPEM...),
		KeyPEM:  pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}),
	}
}

// create signs a certificate. Ed25519 signatures need no randomness, so
// the result depends only on the inputs. On an error it calls g.fail and
// returns nil values.
func (g *Generator) create(
	tmpl, parent *x509.Certificate, pub any, signer ed25519.PrivateKey,
) (*x509.Certificate, []byte) {
	der, err := x509.CreateCertificate(nil, tmpl, parent, pub, signer)
	if err != nil {
		g.fail(err)
		return nil, nil
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		g.fail(err)
		return nil, nil
	}
	return cert, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}
