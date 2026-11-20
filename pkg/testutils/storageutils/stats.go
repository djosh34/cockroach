// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package storageutils

import (
	"testing"

	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/storage"
	"github.com/cockroachdb/cockroach/pkg/storage/enginepb"
	"github.com/stretchr/testify/require"
)

// EngineStats computes MVCC stats for the given engine reader, limited to the
// replicated user keyspace.
func EngineStats(t *testing.T, engine storage.Reader, nowNanos int64) *enginepb.MVCCStats {
	t.Helper()

	stats, err := storage.ComputeStats(engine, keys.LocalMax, keys.MaxKey, nowNanos)
	require.NoError(t, err)
	return &stats
}

// SSTStats computes MVCC stats for the given binary SST.
func SSTStats(t *testing.T, sst []byte, nowNanos int64) *enginepb.MVCCStats {
	t.Helper()

	iter, err := storage.NewMemSSTIterator(sst, true /* verify */, storage.IterOptions{
		KeyTypes:   storage.IterKeyTypePointsAndRanges,
		LowerBound: keys.MinKey,
		UpperBound: keys.MaxKey,
	})
	require.NoError(t, err)
	defer iter.Close()
	iter.SeekGE(storage.MVCCKey{Key: keys.MinKey})
	stats, err := storage.ComputeStatsForIter(iter, nowNanos)
	require.NoError(t, err)
	return &stats
}
