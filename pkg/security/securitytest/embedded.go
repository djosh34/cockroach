// Copyright 2026 The Cockroach Authors.
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
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/cockroach/pkg/security/certconfig"
	"github.com/cockroachdb/cockroach/pkg/security/securityassets"
	"github.com/cockroachdb/errors/oserror"
)

// Seed is the seed of the default test certificates.
const Seed = 1

// TestUsers have a client cert and a client rule in the default test
// certificates, so that tests can log in as them. A test can still read a
// client cert for any other user, but no rule admits it.
var TestUsers = []string{
	"root", "testuser", "testuser2", "testuser3", "testadmin",
	"authentic_user", "authentic_user_noadmin",
	"foo", "bar", "baz", "newuser", "user1", "user2", "adminuser",
	"test_user_a", "test_user_b", "test_user_c", "123foo", "foo.foo",
}

// defaultCAs makes the default test PKI: one root CA, and one intermediate
// that signs every leaf. Both CAs in certconfig.Embedded() are the root.
func defaultCAs(fail func(error)) (root, inter *CA) {
	g := NewGenerator(Seed, fail)
	root = g.RootCA("patchroaddb test CA")
	return root, root.IntermediateCA("patchroaddb test intermediate CA")
}

// embedded caches the default test files.
var embedded struct {
	once  sync.Once
	files map[string][]byte
	err   error
}

func initEmbedded() error {
	embedded.once.Do(func() {
		fail := func(err error) {
			if embedded.err == nil {
				embedded.err = err
			}
		}
		root, inter := defaultCAs(fail)
		node := inter.NodeCert(
			Identity{CN: "node", SPIFFE: SPIFFEID("node")}, "localhost", "127.0.0.1", "::1")
		rules := "# Default test rules.\n" +
			"node cn=node spiffe=" + SPIFFEID("node") + "\n"
		files := map[string][]byte{
			"ca.crt":   root.CertPEM,
			"node.crt": node.CertPEM,
			"node.key": node.KeyPEM,
		}
		for _, u := range TestUsers {
			c := inter.ClientCert(Identity{CN: u})
			files["client."+u+".crt"] = c.CertPEM
			files["client."+u+".key"] = c.KeyPEM
			rules += "client cn=" + u + " role=" + u + "\n"
		}
		files["rules"] = []byte(rules)
		embedded.files = files
	})
	return embedded.err
}

// File returns a default test file by name, such as "ca.crt" or
// "client.root.key". It makes client certs for users not in TestUsers on
// demand. It returns an error that oserror.IsNotExist accepts for an
// unknown name.
func File(name string) ([]byte, error) {
	if err := initEmbedded(); err != nil {
		return nil, err
	}
	if b, ok := embedded.files[name]; ok {
		return b, nil
	}
	user, ok := strings.CutPrefix(name, "client.")
	if !ok {
		return nil, oserror.ErrNotExist
	}
	var err error
	_, inter := defaultCAs(func(e error) {
		if err == nil {
			err = e
		}
	})
	if u, ok := strings.CutSuffix(user, ".crt"); ok {
		b := inter.ClientCert(Identity{CN: u}).CertPEM
		return b, err
	}
	if u, ok := strings.CutSuffix(user, ".key"); ok {
		b := inter.ClientCert(Identity{CN: u}).KeyPEM
		return b, err
	}
	return nil, oserror.ErrNotExist
}

// embeddedName returns the file name for a path in certconfig.EmbeddedDir.
func embeddedName(path string) (string, bool) {
	return strings.CutPrefix(filepath.Clean(path), certconfig.EmbeddedDir+string(filepath.Separator))
}

// EmbeddedAssets serves the default test files under certconfig.EmbeddedDir
// from memory, and every other path from disk.
var EmbeddedAssets = securityassets.Loader{
	ReadFile: func(path string) ([]byte, error) {
		name, ok := embeddedName(path)
		if !ok {
			return os.ReadFile(path)
		}
		b, err := File(name)
		if err != nil {
			return nil, &fs.PathError{Op: "read", Path: path, Err: err}
		}
		return b, nil
	},
	Stat: func(path string) (os.FileInfo, error) {
		name, ok := embeddedName(path)
		if !ok {
			return os.Stat(path)
		}
		b, err := File(name)
		if err != nil {
			return nil, &fs.PathError{Op: "stat", Path: path, Err: err}
		}
		return fileInfo{name: name, size: int64(len(b))}, nil
	},
	InMemory: func(path string) bool {
		_, ok := embeddedName(path)
		return ok
	},
}

// fileInfo describes an in-memory file: a regular file only its owner can
// read.
type fileInfo struct {
	name string
	size int64
}

func (i fileInfo) Name() string       { return i.name }
func (i fileInfo) Size() int64        { return i.size }
func (i fileInfo) Mode() fs.FileMode  { return 0600 }
func (i fileInfo) ModTime() time.Time { return time.Time{} }
func (i fileInfo) IsDir() bool        { return false }
func (i fileInfo) Sys() any           { return nil }
