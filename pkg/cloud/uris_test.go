// Copyright 2021 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package cloud_test

import (
	"testing"

	"github.com/cockroachdb/cockroach/pkg/cloud"
	"github.com/stretchr/testify/require"
)

func TestSanitizeExternalStorageURI(t *testing.T) {
	// Register a scheme to test scheme-specific redaction.
	cloud.RegisterExternalStorageProvider(0, nil, nil,
		cloud.RedactedParams("TEST_PARAM"),
		"test-scheme",
	)
	testCases := []struct {
		name             string
		inputURI         string
		inputExtraParams []string
		expected         string
	}{
		{
			name:     "redacts password",
			inputURI: "http://username:password@foo.com/something",
			expected: "http://username:redacted@foo.com/something",
		},
		{
			name:             "redacts given parameters",
			inputURI:         "http://foo.com/something?secret_key=uhoh",
			inputExtraParams: []string{"secret_key"},
			expected:         "http://foo.com/something?secret_key=redacted",
		},
		{
			name:     "redacts registered parameters",
			inputURI: "test-scheme://somehost/somepath?TEST_PARAM=uhoh",
			expected: "test-scheme://somehost/somepath?TEST_PARAM=redacted",
		},
		{
			name:     "preserves username",
			inputURI: "http://username@foo.com/something",
			expected: "http://username@foo.com/something",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualOutput, err := cloud.SanitizeExternalStorageURI(tc.inputURI, tc.inputExtraParams)
			require.NoError(t, err)
			require.Equal(t, tc.expected, actualOutput)
		})
	}
}

func TestParseURI(t *testing.T) {
	for _, tc := range []struct {
		uri, host, path, err string
	}{
		{uri: "https://a:1/p", host: "a:1", path: "/p"},
		{uri: "https://a:1,b:2,c/p?x=1", host: "a:1,b:2,c", path: "/p"},
		{uri: "http://u:pw@a:1,b:2", host: "a:1,b:2"},
		{uri: "https://a:1:2/p", err: "invalid port"},
		{uri: "https://a:1,b:2:3/p", err: "invalid port"},
		{uri: "nodelocal://1,2/p", host: "1,2", path: "/p"},
		// A zone ID is escaped as %25 in the URI and decoded in Host.
		{uri: "https://[fe80::1%25eth0]:1,[fe80::2%25eth0]:2/p",
			host: "[fe80::1%eth0]:1,[fe80::2%eth0]:2", path: "/p"},
	} {
		t.Run(tc.uri, func(t *testing.T) {
			u, err := cloud.ParseURI(tc.uri)
			if tc.err != "" {
				require.ErrorContains(t, err, tc.err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.host, u.Host)
			require.Equal(t, tc.path, u.Path)
			require.Equal(t, tc.uri, u.String())
			// Parsing the serialized URI gives the same URI again.
			again, err := cloud.ParseURI(u.String())
			require.NoError(t, err)
			require.Equal(t, tc.uri, again.String())
		})
	}
}
