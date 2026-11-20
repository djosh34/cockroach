// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package build

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestComputeBinaryVersion(t *testing.T) {
	testCases := []struct {
		name            string
		versionTxt      string
		revision        string
		buildType       string
		panicExpected   bool
		expectedVersion string
	}{
		{
			name:          "empty version.txt",
			versionTxt:    "",
			revision:      "abc123",
			panicExpected: true,
		},
		{
			name:          "invalid version.txt",
			versionTxt:    "vInvalid.23",
			revision:      "abc123",
			panicExpected: true,
		},
		{
			name:            "dev release",
			versionTxt:      "v21.2.0",
			revision:        "abc123",
			buildType:       "development",
			expectedVersion: "v21.2.0-dev-abc123",
		},
		{
			name:            "release binary",
			versionTxt:      "v21.2.0",
			revision:        "abc123",
			buildType:       "release",
			expectedVersion: "v21.2.0",
		},
		{
			name:            "dev pre-release",
			versionTxt:      "v21.2.0-alpha.2",
			revision:        "abc123",
			buildType:       "development",
			expectedVersion: "v21.2.0-alpha.2-dev-abc123",
		},
		{
			name:            "pre-release binary",
			versionTxt:      "v21.2.0-alpha.2",
			revision:        "abc123",
			buildType:       "release",
			expectedVersion: "v21.2.0-alpha.2",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			oldBuildType := typ
			typ = tc.buildType
			defer func() { typ = oldBuildType }()

			if tc.panicExpected {
				require.Panics(t, func() { computeBinaryVersion(tc.versionTxt, tc.revision) })
			} else {
				actualVersion := computeBinaryVersion(tc.versionTxt, tc.revision)
				require.Equal(t, tc.expectedVersion, actualVersion)
			}
		})
	}
}

// TestVersionNamesOnlyPatchroaddb checks that the version strings that
// version(), crdb_internal.node_build_info and 'patchroaddb version' show
// name patchroaddb and its version.txt version, and no CockroachDB version.
func TestVersionNamesOnlyPatchroaddb(t *testing.T) {
	info := GetInfo()
	require.True(t, strings.HasPrefix(info.Tag, strings.TrimSpace(cockroachVersion)), info.Tag)
	require.True(t, strings.HasPrefix(info.Short(), "patchroaddb "+info.Tag+" ("), info.Short())
	require.True(t, strings.HasPrefix(info.Long(), "patchroaddb "+info.Tag+"\n"), info.Long())
	for _, s := range []string{info.Short(), info.Long()} {
		require.NotContains(t, s, "CockroachDB")
		require.NotContains(t, s, "23.2")
	}
}
