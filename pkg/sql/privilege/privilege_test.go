// Copyright 2015 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package privilege_test

import (
	"bytes"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/sql/privilege"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/redact"
	"github.com/stretchr/testify/require"
)

func TestPrivilegeDecode(t *testing.T) {
	defer leaktest.AfterTest(t)()

	testCases := []struct {
		raw        uint64
		privileges privilege.List
	}{
		{0, privilege.List{}},
		// We avoid 0 as a privilege value even though we use 1 << privValue.
		{1, privilege.List{}},
		{2, privilege.List{privilege.ALL}},
		{10, privilege.List{privilege.ALL, privilege.DROP}},
		{384, privilege.List{privilege.DELETE, privilege.UPDATE}},
		{2046, privilege.List{privilege.ALL, privilege.CREATE, privilege.DROP,
			privilege.SELECT, privilege.INSERT, privilege.DELETE, privilege.UPDATE, privilege.USAGE, privilege.ZONECONFIG},
		},
	}

	for _, tc := range testCases {
		pl, err := privilege.ListFromBitField(tc.raw, privilege.Any)
		if err != nil {
			t.Fatal(err)
		}
		require.Equal(t, tc.privileges, pl)
	}
}

func TestPrivilegeListFormat(t *testing.T) {
	defer leaktest.AfterTest(t)()

	testCases := []struct {
		privileges         privilege.List
		redactable         redact.RedactableString
		formatNames        string
		sortedDisplayNames []string
		sortedKeys         []string
	}{
		{privilege.List{}, "", "", []string{}, []string{}},
		{privilege.List{privilege.ALL}, "ALL", "ALL", []string{"ALL"}, []string{"ALL"}},
		{privilege.List{privilege.UPDATE, privilege.DELETE}, "UPDATE, DELETE", "UPDATE, DELETE", []string{"DELETE", "UPDATE"}, []string{"DELETE", "UPDATE"}},
	}

	for _, tc := range testCases {
		require.Equal(t, tc.redactable, redact.Sprint(tc.privileges))

		var buf bytes.Buffer
		tc.privileges.FormatNames(&buf)
		require.Equal(t, tc.formatNames, buf.String())

		require.Equal(t, tc.sortedDisplayNames, tc.privileges.SortedDisplayNames())
		require.Equal(t, tc.sortedKeys, tc.privileges.SortedKeys())
	}
}

// TestByDisplayNameHasAllPrivileges verifies that every privilege is present in
// ByDisplayName.
func TestByDisplayNameHasAllPrivileges(t *testing.T) {
	defer leaktest.AfterTest(t)()

	for _, kind := range privilege.AllPrivileges {
		resolvedKind, ok := privilege.ByDisplayName[kind.DisplayName()]
		require.True(t, ok)
		require.Equal(t, kind, resolvedKind)

		// It must also be possible to resolve the privilege using its
		// internal key as input.
		resolvedKind, ok = privilege.ByDisplayName[privilege.KindDisplayName(kind.InternalKey())]
		require.True(t, ok)
		require.Equal(t, kind, resolvedKind)
	}
}
