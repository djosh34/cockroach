// Copyright 2017 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package sql

import (
	"bytes"
	"sort"

	"github.com/cockroachdb/cockroach/pkg/config/zonepb"
	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/util/uuid"
)

// GenerateSubzoneSpans constructs from a TableDescriptor the entries mapping
// zone config spans to subzones for use in the SubzoneSpans field of
// zonepb.ZoneConfig. SubzoneSpans controls which splits are created, so only
// the spans corresponding to entries in subzones are returned.
//
// Zone configs target indexes via `subzones`, which are attached to a
// table-scoped row in `system.zones`. Each subzone represents one index
// (primary or secondary) and contains the usual zone config constraints. They
// are saved to `system.zones` sparsely (only when set by a user) and are the
// most specific entry in the normal cluster-default/database/table/subzone
// config hierarchy. Subzones that name a partition are ignored.
//
// The returned spans are returned in exactly the format required by
// `system.zones`. They are sorted and non-overlapping. Each contains an
// IndexID, which maps to one of the input `subzones` by indexing into the
// slice. As space optimizations, all `Key`s and `EndKey`s of `SubzoneSpan` omit
// the common prefix (the encoded table ID) and if `EndKey` is equal to
// `Key.PrefixEnd()` it is omitted.
//
// TODO(benesch): remove the hasNewSubzones parameter when a statement to clear
// all subzones at once is introduced.
func GenerateSubzoneSpans(
	st *cluster.Settings,
	logicalClusterID uuid.UUID,
	codec keys.SQLCodec,
	tableDesc catalog.TableDescriptor,
	subzones []zonepb.Subzone,
	hasNewSubzones bool,
) ([]zonepb.SubzoneSpan, error) {
	// We already completely avoid creating subzone spans for dropped indexes.
	// Whether this was intentional is a different story, but it turns out to be
	// pretty sane. Dropped elements may refer to dropped types and we aren't
	// necessarily in a position to deal with those dropped types. Add a special
	// case to avoid generating any subzone spans in the face of being dropped.
	if tableDesc.Dropped() {
		return nil, nil
	}

	subzoneIndexByIndexID := make(map[descpb.IndexID]int32)
	for i, subzone := range subzones {
		if len(subzone.PartitionName) == 0 {
			subzoneIndexByIndexID[descpb.IndexID(subzone.IndexID)] = int32(i)
		}
	}

	// NB: This assumes that none of the indexes are interleaved.
	sharedPrefix := codec.TablePrefix(uint32(tableDesc.GetID()))

	var subzoneSpans []zonepb.SubzoneSpan
	if err := catalog.ForEachIndex(tableDesc, catalog.IndexOpts{
		AddMutations: true,
	}, func(idx catalog.Index) error {
		subzoneIndex, ok := subzoneIndexByIndexID[idx.GetID()]
		if !ok {
			return nil
		}
		idxSpan := tableDesc.IndexSpan(codec, idx.GetID())
		subzoneSpan := zonepb.SubzoneSpan{
			Key:          bytes.TrimPrefix(idxSpan.Key, sharedPrefix),
			EndKey:       bytes.TrimPrefix(idxSpan.EndKey, sharedPrefix),
			SubzoneIndex: subzoneIndex,
		}
		if bytes.Equal(subzoneSpan.Key.PrefixEnd(), subzoneSpan.EndKey) {
			subzoneSpan.EndKey = nil
		}
		subzoneSpans = append(subzoneSpans, subzoneSpan)
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Slice(subzoneSpans, func(i, j int) bool {
		return subzoneSpans[i].Key.Compare(subzoneSpans[j].Key) < 0
	})
	return subzoneSpans, nil
}
