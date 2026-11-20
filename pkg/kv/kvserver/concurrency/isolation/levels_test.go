// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package isolation

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLevel_WeakerThan(t *testing.T) {
	tests := []struct {
		l1, l2 Level
		exp    bool
	}{
		{Serializable, Serializable, false},
		{Serializable, Snapshot, false},
		{Serializable, ReadCommitted, false},
		{Snapshot, Serializable, true},
		{Snapshot, Snapshot, false},
		{Snapshot, ReadCommitted, false},
		{ReadCommitted, Serializable, true},
		{ReadCommitted, Snapshot, true},
		{ReadCommitted, ReadCommitted, false},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s<%s", tt.l1, tt.l2), func(t *testing.T) {
			require.Equal(t, tt.exp, tt.l1.WeakerThan(tt.l2))
		})
	}
}

func TestLevel_ToleratesWriteSkew(t *testing.T) {
	exp := map[Level]bool{
		Serializable:  false,
		Snapshot:      true,
		ReadCommitted: true,
	}
	for l, exp := range exp {
		t.Run(l.String(), func(t *testing.T) {
			require.Equal(t, exp, l.ToleratesWriteSkew())
		})
	}
}

func TestLevel_PerStatementReadSnapshot(t *testing.T) {
	exp := map[Level]bool{
		Serializable:  false,
		Snapshot:      false,
		ReadCommitted: true,
	}
	for l, exp := range exp {
		t.Run(l.String(), func(t *testing.T) {
			require.Equal(t, exp, l.PerStatementReadSnapshot())
		})
	}
}

func TestLevel_String(t *testing.T) {
	exp := map[Level]string{
		Serializable:  "Serializable",
		Snapshot:      "Snapshot",
		ReadCommitted: "ReadCommitted",
	}
	for l, exp := range exp {
		t.Run(l.String(), func(t *testing.T) {
			require.Equal(t, exp, l.String())
		})
	}
}

func TestLevel_StringLower(t *testing.T) {
	exp := map[Level]string{
		Serializable:  "serializable",
		Snapshot:      "snapshot",
		ReadCommitted: "read committed",
	}
	for l, exp := range exp {
		t.Run(l.String(), func(t *testing.T) {
			require.Equal(t, exp, l.StringLower())
		})
	}
}
