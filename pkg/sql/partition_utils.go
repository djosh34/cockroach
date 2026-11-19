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
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/covering"
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
func storedPartitionSpans(z *zonepb.ZoneConfig, oldRaw []byte) ([]zonepb.SubzoneSpan, error) {
	if len(oldRaw) == 0 {
		return nil, nil
	}
	var old zonepb.ZoneConfig
	if err := protoutil.Unmarshal(oldRaw, &old); err != nil {
		return nil, err
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
		for i, subzone := range z.Subzones {
			if subzone.IndexID == oldSubzone.IndexID && subzone.PartitionName == oldSubzone.PartitionName {
				s.SubzoneIndex = int32(i)
				spans = append(spans, s)
				break
			}
		}
	}
	return spans, nil
}
