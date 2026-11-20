// Copyright 2026 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package sql

import (
	"bytes"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/config/zonepb"
	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/systemschema"
	"github.com/cockroachdb/cockroach/pkg/util/encoding"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/protoutil"
	"github.com/stretchr/testify/require"
)

// TestSubzoneSpansKeepStoredPartitionSpans checks that a zone config write
// keeps the stored spans of partition subzones, which SQL can no longer
// compute itself.
func TestSubzoneSpansKeepStoredPartitionSpans(t *testing.T) {
	defer leaktest.AfterTest(t)()

	codec := keys.SystemSQLCodec
	table := systemschema.CommentsTable
	tableID := uint32(table.GetID())
	tablePrefix := codec.TablePrefix(tableID)
	trim := func(k roachpb.Key) roachpb.Key {
		return roachpb.Key(bytes.TrimPrefix(k, tablePrefix))
	}
	idxStart := trim(codec.IndexPrefix(tableID, 1))
	idxEnd := trim(codec.IndexPrefix(tableID, 1).PrefixEnd())
	partStart := encoding.EncodeVarintAscending(append(roachpb.Key(nil), idxStart...), 1)
	partEnd := encoding.EncodeVarintAscending(append(roachpb.Key(nil), idxStart...), 5)

	// The stored zone config has one partition subzone and its span.
	stored := zonepb.ZoneConfig{
		Subzones:     []zonepb.Subzone{{IndexID: 1, PartitionName: "p1"}},
		SubzoneSpans: []zonepb.SubzoneSpan{{Key: partStart, EndKey: partEnd, SubzoneIndex: 0}},
	}
	storedRaw, err := protoutil.Marshal(&stored)
	require.NoError(t, err)

	generate := func(z *zonepb.ZoneConfig) []zonepb.SubzoneSpan {
		partitionSpans, err := storedPartitionSpans(z, storedRaw, nil /* indexRemap */)
		require.NoError(t, err)
		spans, err := GenerateSubzoneSpans(codec, table, z.Subzones, partitionSpans)
		require.NoError(t, err)
		return spans
	}

	// Configuring the whole primary index keeps the partition span, which takes
	// precedence over the index span it overlaps.
	updated := zonepb.ZoneConfig{Subzones: []zonepb.Subzone{
		{IndexID: 1},
		{IndexID: 1, PartitionName: "p1"},
	}}
	require.Equal(t, []zonepb.SubzoneSpan{
		{Key: idxStart, EndKey: partStart, SubzoneIndex: 0},
		{Key: partStart, EndKey: partEnd, SubzoneIndex: 1},
		{Key: partEnd, EndKey: idxEnd, SubzoneIndex: 0},
	}, generate(&updated))

	// Removing the partition subzone removes its span.
	withoutPartition := zonepb.ZoneConfig{Subzones: []zonepb.Subzone{{IndexID: 1}}}
	require.Equal(t, []zonepb.SubzoneSpan{
		{Key: idxStart, SubzoneIndex: 0},
	}, generate(&withoutPartition))

	// A rewritten index takes the partition span along, with the index prefix
	// swapped. The span of the old index stays while its subzone exists.
	movedStart := encoding.EncodeVarintAscending(encoding.EncodeUvarintAscending(nil, 7), 1)
	movedEnd := encoding.EncodeVarintAscending(encoding.EncodeUvarintAscending(nil, 7), 5)
	rewritten := zonepb.ZoneConfig{Subzones: []zonepb.Subzone{
		{IndexID: 1, PartitionName: "p1"},
		{IndexID: 7, PartitionName: "p1"},
	}}
	spans, err := storedPartitionSpans(
		&rewritten, storedRaw, map[descpb.IndexID]descpb.IndexID{1: 7},
	)
	require.NoError(t, err)
	require.Equal(t, []zonepb.SubzoneSpan{
		{Key: partStart, EndKey: partEnd, SubzoneIndex: 0},
		{Key: movedStart, EndKey: movedEnd, SubzoneIndex: 1},
	}, spans)

	// A span that ends at the end of the old index ends at the end of the new
	// one.
	moved, ok := moveSpanToIndex(zonepb.SubzoneSpan{Key: partStart, EndKey: idxEnd}, 1, 7)
	require.True(t, ok)
	require.Equal(t, roachpb.Key(encoding.EncodeUvarintAscending(nil, 7)).PrefixEnd(), moved.EndKey)
}
