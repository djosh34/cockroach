// Copyright 2026 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

// Package certconfig reads the --config file. The file names the PEM files
// that patchroaddb uses for TLS and the identity rules file. patchroaddb
// never writes any of these files.
//
// Example:
//
//	cert: node.crt          # node cert, leaf first, then intermediates
//	key: node.key
//	node-ca: node-ca.crt    # trusted for node-to-node connections
//	client-ca: client-ca.crt # trusted for SQL and HTTP clients
//	rules: rules            # identity rules
//	client-cert: client.crt # optional, used by the CLI for SQL
//	client-key: client.key
//
// Relative paths are relative to the directory of the config file.
package certconfig

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/cockroachdb/errors"
	"gopkg.in/yaml.v3"
)

// Config holds the paths from the --config file.
type Config struct {
	Cert       string `yaml:"cert"`
	Key        string `yaml:"key"`
	NodeCA     string `yaml:"node-ca"`
	ClientCA   string `yaml:"client-ca"`
	Rules      string `yaml:"rules"`
	ClientCert string `yaml:"client-cert"`
	ClientKey  string `yaml:"client-key"`
}

// Load reads and checks the config file at path.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, errors.Wrap(err, "reading config file")
	}
	var c Config
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil {
		return Config{}, errors.Wrapf(err, "parsing config file %s", path)
	}
	dir := filepath.Dir(path)
	for _, p := range c.paths() {
		if *p != "" && !filepath.IsAbs(*p) {
			*p = filepath.Join(dir, *p)
		}
	}
	if (c.ClientCert == "") != (c.ClientKey == "") {
		return Config{}, errors.Newf("config file %s: client-cert and client-key go together", path)
	}
	return c, nil
}

func (c *Config) paths() []*string {
	return []*string{&c.Cert, &c.Key, &c.NodeCA, &c.ClientCA, &c.Rules, &c.ClientCert, &c.ClientKey}
}

// CheckNode returns an error if a field that a node needs is empty.
func (c Config) CheckNode() error {
	for _, f := range []struct{ name, value string }{
		{"cert", c.Cert}, {"key", c.Key}, {"node-ca", c.NodeCA},
		{"client-ca", c.ClientCA}, {"rules", c.Rules},
	} {
		if f.value == "" {
			return errors.Newf("the config file has no %q path", f.name)
		}
	}
	return nil
}

// CheckSQLClient returns an error if a field that a SQL client needs is
// empty.
func (c Config) CheckSQLClient() error {
	if c.ClientCert == "" || c.NodeCA == "" {
		return errors.New("the config file needs client-cert, client-key and node-ca to connect to SQL")
	}
	return nil
}

// EmbeddedDir is the directory of the test certificates that the
// securitytest package serves from memory.
const EmbeddedDir = "test_certs"

// Embedded returns the config of the test certificates that the
// securitytest package serves from memory. Both CAs are the same file, the
// client cert is root's.
func Embedded() Config {
	p := func(name string) string { return filepath.Join(EmbeddedDir, name) }
	return Config{
		Cert:       p("node.crt"),
		Key:        p("node.key"),
		NodeCA:     p("ca.crt"),
		ClientCA:   p("ca.crt"),
		Rules:      p("rules"),
		ClientCert: p("client.root.crt"),
		ClientKey:  p("client.root.key"),
	}
}
