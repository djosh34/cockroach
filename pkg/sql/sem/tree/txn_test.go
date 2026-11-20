// Copyright 2015 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package tree_test

import (
	"testing"

	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/concurrency/isolation"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/stretchr/testify/require"
)

func TestIsolationLevelFromKVTxnIsolationLevel(t *testing.T) {
	defer leaktest.AfterTest(t)()

	testCases := []struct {
		In  isolation.Level
		Out tree.IsolationLevel
	}{
		{
			In:  isolation.Serializable,
			Out: tree.SerializableIsolation,
		},
		{
			In:  isolation.ReadCommitted,
			Out: tree.ReadCommittedIsolation,
		},
		{
			In:  isolation.Snapshot,
			Out: tree.SnapshotIsolation,
		},
	}

	for _, tc := range testCases {
		require.Equal(t, tc.Out, tree.IsolationLevelFromKVTxnIsolationLevel(tc.In))
	}
}
