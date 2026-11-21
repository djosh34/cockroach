// Copyright 2026 The Cockroach Authors.
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
	"bufio"
	"bytes"
	"crypto/x509"
	"fmt"
	"strconv"
	"strings"

	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/errors"
)

// Rules are the identity rules from the rules file. Each line admits
// certificates by exact CN and SPIFFE ID:
//
//	node cn=<CN> spiffe=<SPIFFE ID>
//	client cn=<CN> spiffe=<SPIFFE ID> role=<SQL role>
//
// A line lists cn, spiffe or both, and matches a certificate only if every
// key it lists matches exactly. A client line also names the role it maps
// to. Values that contain spaces go in double quotes. Empty lines and lines
// starting with # are ignored.
type Rules struct {
	rules []rule
}

type ruleKind int

const (
	nodeRule ruleKind = iota
	clientRule
)

type rule struct {
	kind   ruleKind
	cn     *string
	spiffe *string
	role   username.SQLUsername
}

// ParseRules parses a rules file.
func ParseRules(data []byte) (*Rules, error) {
	var r Rules
	s := bufio.NewScanner(bytes.NewReader(data))
	for n := 1; s.Scan(); n++ {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		ru, err := parseRule(line)
		if err != nil {
			return nil, errors.Wrapf(err, "rules line %d", n)
		}
		r.rules = append(r.rules, ru)
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return &r, nil
}

func parseRule(line string) (rule, error) {
	words, err := splitWords(line)
	if err != nil {
		return rule{}, err
	}
	var ru rule
	switch words[0] {
	case "node":
		ru.kind = nodeRule
	case "client":
		ru.kind = clientRule
	default:
		return rule{}, errors.Newf("line must start with node or client, not %q", words[0])
	}
	var role *string
	for _, w := range words[1:] {
		key, value, ok := strings.Cut(w, "=")
		if !ok || value == "" {
			return rule{}, errors.Newf("%q is not key=value", w)
		}
		var dst **string
		switch key {
		case "cn":
			dst = &ru.cn
		case "spiffe":
			if !strings.HasPrefix(value, "spiffe://") {
				return rule{}, errors.Newf("spiffe=%s does not start with spiffe://", value)
			}
			dst = &ru.spiffe
		case "role":
			if ru.kind == nodeRule {
				return rule{}, errors.New("a node line has no role")
			}
			dst = &role
		default:
			return rule{}, errors.Newf("unknown key %q", key)
		}
		if *dst != nil {
			return rule{}, errors.Newf("key %q given twice", key)
		}
		*dst = &value
	}
	if ru.cn == nil && ru.spiffe == nil {
		return rule{}, errors.New("a line needs cn, spiffe or both")
	}
	if ru.kind == clientRule {
		if role == nil {
			return rule{}, errors.New("a client line needs a role")
		}
		ru.role, err = username.MakeSQLUsernameFromUserInput(*role, username.PurposeValidation)
		if err != nil {
			return rule{}, err
		}
		if ru.role.IsNodeUser() {
			return rule{}, errors.Newf("role %s is reserved", ru.role)
		}
	}
	return ru, nil
}

// splitWords splits a line on spaces. A value may be double quoted, as in
// cn="Jane Doe".
func splitWords(line string) ([]string, error) {
	var words []string
	for line != "" {
		end := strings.IndexAny(line, " \t\"")
		if end >= 0 && line[end] == '"' {
			q, err := strconv.QuotedPrefix(line[end:])
			if err != nil {
				return nil, errors.Newf("bad quoted value in %q", line)
			}
			v, _ := strconv.Unquote(q)
			rest := line[end+len(q):]
			if rest != "" && rest[0] != ' ' && rest[0] != '\t' {
				return nil, errors.Newf("text after quoted value in %q", line)
			}
			words = append(words, line[:end]+v)
			line = strings.TrimLeft(rest, " \t")
			continue
		}
		if end < 0 {
			end = len(line)
		}
		words = append(words, line[:end])
		line = strings.TrimLeft(line[end:], " \t")
	}
	return words, nil
}

// String formats a line in the form ParseRules reads.
func (ru rule) String() string {
	var b strings.Builder
	if ru.kind == nodeRule {
		b.WriteString("node")
	} else {
		b.WriteString("client")
	}
	word := func(key, value string) {
		if strings.ContainsAny(value, " \t\"") {
			value = strconv.Quote(value)
		}
		fmt.Fprintf(&b, " %s=%s", key, value)
	}
	if ru.cn != nil {
		word("cn", *ru.cn)
	}
	if ru.spiffe != nil {
		word("spiffe", *ru.spiffe)
	}
	if ru.kind == clientRule {
		word("role", ru.role.Normalized())
	}
	return b.String()
}

func (ru rule) matches(cert *x509.Certificate) bool {
	if ru.cn != nil && cert.Subject.CommonName != *ru.cn {
		return false
	}
	if ru.spiffe != nil {
		id, ok := SPIFFEID(cert)
		if !ok || id != *ru.spiffe {
			return false
		}
	}
	return true
}

// SPIFFEID returns the SPIFFE ID of a certificate. A certificate has one if
// its only URI name has the spiffe scheme, as the X509-SVID standard asks.
func SPIFFEID(cert *x509.Certificate) (string, bool) {
	if len(cert.URIs) != 1 || cert.URIs[0].Scheme != "spiffe" {
		return "", false
	}
	return cert.URIs[0].String(), true
}

// MatchesNode reports whether a node line matches the certificate.
func (r *Rules) MatchesNode(cert *x509.Certificate) bool {
	for _, ru := range r.rules {
		if ru.kind == nodeRule && ru.matches(cert) {
			return true
		}
	}
	return false
}

// ClientRoles returns the roles of the client lines that match the
// certificate, in file order.
func (r *Rules) ClientRoles(cert *x509.Certificate) []username.SQLUsername {
	var roles []username.SQLUsername
	for _, ru := range r.rules {
		if ru.kind == clientRule && ru.matches(cert) {
			roles = append(roles, ru.role)
		}
	}
	return roles
}

// Lines returns the rules, one line each, in the form ParseRules reads.
func (r *Rules) Lines() []string {
	lines := make([]string, len(r.rules))
	for i, ru := range r.rules {
		lines[i] = ru.String()
	}
	return lines
}

// DescribeCert names the CN and SPIFFE ID of a certificate, for errors.
func DescribeCert(cert *x509.Certificate) string {
	id, ok := SPIFFEID(cert)
	if !ok {
		id = "none"
	}
	return fmt.Sprintf("cn=%q spiffe=%s", cert.Subject.CommonName, id)
}
