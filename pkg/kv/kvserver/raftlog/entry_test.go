// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.
//

package raftlog

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.etcd.io/raft/v3/raftpb"
)

func TestNewEntry(t *testing.T) {
	// TODO(replication): Add more cases.
	testcases := map[string]struct {
		data        []byte
		expectEmpty bool
		expectErr   bool
	}{
		// Proposed by Raft on leader change.
		"empty entry": {data: nil, expectEmpty: true},
		// Proposed by CRDB on unquiescence.
		"empty payload": {
			data:        EncodeCommandBytes(EntryEncodingStandardWithoutAC, "00000000", nil),
			expectEmpty: true,
		},
		"invalid": {
			data:      EncodeCommandBytes(EntryEncodingStandardWithAC, "00000000", []byte("not a protobuf")),
			expectErr: true,
		},
	}
	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			ent, err := NewEntry(raftpb.Entry{
				Term:  1,
				Index: 1,
				Data:  tc.data,
			})
			if tc.expectErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			// Clear out the passed Raft entry, and only assert on the decoded entry.
			require.NotNil(t, ent)
			ent.Entry = raftpb.Entry{}
			if tc.expectEmpty {
				require.Zero(t, *ent)
			} else {
				require.NotZero(t, *ent)
			}
		})
	}
}
