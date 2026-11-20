// Copyright 2017 The Cockroach Authors.
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
	"sort"

	"github.com/cockroachdb/cockroach/pkg/config/zonepb"
	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/covering"
	"github.com/cockroachdb/cockroach/pkg/util/encoding"
	"github.com/cockroachdb/cockroach/pkg/util/protoutil"
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
// config hierarchy.
//
// SQL cannot create partitions, so it cannot compute the spans of subzones
// that name a partition. Such subzones only exist in stored zone configs.
// partitionSpans holds their stored spans, with SubzoneIndex already pointing
// into subzones. They are kept as they are and take precedence over the index
// spans they overlap.
//
// The returned spans are returned in exactly the format required by
// `system.zones`. They are sorted and non-overlapping. Each contains an
// IndexID, which maps to one of the input `subzones` by indexing into the
// slice. As space optimizations, all `Key`s and `EndKey`s of `SubzoneSpan` omit
// the common prefix (the encoded table ID) and if `EndKey` is equal to
// `Key.PrefixEnd()` it is omitted.
func GenerateSubzoneSpans(
	codec keys.SQLCodec,
	tableDesc catalog.TableDescriptor,
	subzones []zonepb.Subzone,
	partitionSpans []zonepb.SubzoneSpan,
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

	// All keys below omit the table prefix.
	sharedPrefix := codec.TablePrefix(uint32(tableDesc.GetID()))

	var partitionCovering covering.Covering
	for _, s := range partitionSpans {
		end := s.EndKey
		if len(end) == 0 {
			end = s.Key.PrefixEnd()
		}
		partitionCovering = append(partitionCovering, covering.Range{
			Start: s.Key, End: end, Payload: s.SubzoneIndex,
		})
	}

	var indexCovering covering.Covering
	if err := catalog.ForEachIndex(tableDesc, catalog.IndexOpts{
		AddMutations: true,
	}, func(idx catalog.Index) error {
		subzoneIndex, ok := subzoneIndexByIndexID[idx.GetID()]
		if !ok {
			return nil
		}
		idxSpan := tableDesc.IndexSpan(codec, idx.GetID())
		indexCovering = append(indexCovering, covering.Range{
			Start:   bytes.TrimPrefix(idxSpan.Key, sharedPrefix),
			End:     bytes.TrimPrefix(idxSpan.EndKey, sharedPrefix),
			Payload: subzoneIndex,
		})
		return nil
	}); err != nil {
		return nil, err
	}

	// OverlapCoveringMerge returns the payloads of overlapping coverings in the
	// order the coverings are given, so the partition spans come first.
	ranges := covering.OverlapCoveringMerge([]covering.Covering{partitionCovering, indexCovering})
	var subzoneSpans []zonepb.SubzoneSpan
	for _, r := range ranges {
		payloads := r.Payload.([]interface{})
		if len(payloads) == 0 {
			continue
		}
		subzoneSpan := zonepb.SubzoneSpan{
			Key:          roachpb.Key(r.Start),
			EndKey:       roachpb.Key(r.End),
			SubzoneIndex: payloads[0].(int32),
		}
		if bytes.Equal(subzoneSpan.Key.PrefixEnd(), subzoneSpan.EndKey) {
			subzoneSpan.EndKey = nil
		}
		subzoneSpans = append(subzoneSpans, subzoneSpan)
	}
	sort.Slice(subzoneSpans, func(i, j int) bool {
		return subzoneSpans[i].Key.Compare(subzoneSpans[j].Key) < 0
	})
	return subzoneSpans, nil
}

// storedPartitionSpans returns the subzone spans of the stored zone config
// (oldRaw) that belong to partition subzones which z still has. The returned
// spans point into z.Subzones.
//
// indexRemap maps the IDs of indexes that were rewritten (by TRUNCATE or a
// primary key change) to the IDs of their replacements. A stored partition
// span of a rewritten index is also returned for the same partition on the
// replacement index, with the index prefix swapped, when z has that subzone.
func storedPartitionSpans(
	z *zonepb.ZoneConfig, oldRaw []byte, indexRemap map[descpb.IndexID]descpb.IndexID,
) ([]zonepb.SubzoneSpan, error) {
	if len(oldRaw) == 0 {
		return nil, nil
	}
	var old zonepb.ZoneConfig
	if err := protoutil.Unmarshal(oldRaw, &old); err != nil {
		return nil, err
	}
	findSubzone := func(indexID uint32, partition string) (int32, bool) {
		for i, subzone := range z.Subzones {
			if subzone.IndexID == indexID && subzone.PartitionName == partition {
				return int32(i), true
			}
		}
		return 0, false
	}
	var spans []zonepb.SubzoneSpan
	for _, s := range old.SubzoneSpans {
		if s.SubzoneIndex < 0 || int(s.SubzoneIndex) >= len(old.Subzones) {
			continue
		}
		oldSubzone := old.Subzones[s.SubzoneIndex]
		if oldSubzone.PartitionName == "" {
			continue
		}
		if i, ok := findSubzone(oldSubzone.IndexID, oldSubzone.PartitionName); ok {
			span := s
			span.SubzoneIndex = i
			spans = append(spans, span)
		}
		newIndexID, ok := indexRemap[descpb.IndexID(oldSubzone.IndexID)]
		if !ok {
			continue
		}
		i, ok := findSubzone(uint32(newIndexID), oldSubzone.PartitionName)
		if !ok {
			continue
		}
		if span, ok := moveSpanToIndex(s, descpb.IndexID(oldSubzone.IndexID), newIndexID); ok {
			span.SubzoneIndex = i
			spans = append(spans, span)
		}
	}
	return spans, nil
}

// moveSpanToIndex swaps the index prefix of a subzone span from oldID to
// newID. Subzone span keys are relative to the table prefix, so they start
// with the encoded index ID. It returns false if the span is not inside the
// old index.
func moveSpanToIndex(s zonepb.SubzoneSpan, oldID, newID descpb.IndexID) (zonepb.SubzoneSpan, bool) {
	oldPrefix := roachpb.Key(encoding.EncodeUvarintAscending(nil, uint64(oldID)))
	newPrefix := roachpb.Key(encoding.EncodeUvarintAscending(nil, uint64(newID)))
	if !bytes.HasPrefix(s.Key, oldPrefix) {
		return zonepb.SubzoneSpan{}, false
	}
	moved := s
	moved.Key = append(newPrefix[:len(newPrefix):len(newPrefix)], s.Key[len(oldPrefix):]...)
	switch {
	case len(s.EndKey) == 0:
	case s.EndKey.Equal(oldPrefix.PrefixEnd()):
		moved.EndKey = newPrefix.PrefixEnd()
	case bytes.HasPrefix(s.EndKey, oldPrefix):
		moved.EndKey = append(newPrefix[:len(newPrefix):len(newPrefix)], s.EndKey[len(oldPrefix):]...)
	default:
		return zonepb.SubzoneSpan{}, false
	}
	return moved, true
}

// partitionMovesWithIndex reports whether the partition named partition on
// index oldID describes the same keys on its replacement newID, so that its
// subzone and stored spans can move to the new index.
func partitionMovesWithIndex(
	table catalog.TableDescriptor, oldID, newID descpb.IndexID, partition string,
) bool {
	newIdx := catalog.FindIndexByID(table, newID)
	if newIdx == nil || newIdx.GetPartitioning().FindPartitionByName(partition) == nil {
		return false
	}
	oldIdx := catalog.FindIndexByID(table, oldID)
	if oldIdx == nil {
		// TRUNCATE removes the old index from the descriptor. Its replacement
		// is a copy with the same columns and partitioning.
		return true
	}
	n := oldIdx.GetPartitioning().NumColumns()
	if n == 0 || newIdx.GetPartitioning().NumColumns() != n ||
		oldIdx.NumKeyColumns() < n || newIdx.NumKeyColumns() < n {
		return false
	}
	for i := 0; i < n; i++ {
		if oldIdx.GetKeyColumnID(i) != newIdx.GetKeyColumnID(i) ||
			oldIdx.GetKeyColumnDirection(i) != newIdx.GetKeyColumnDirection(i) {
			return false
		}
	}
	return true
}
