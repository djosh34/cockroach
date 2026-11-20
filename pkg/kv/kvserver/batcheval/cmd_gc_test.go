// Copyright 2026 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package batcheval

import (
	"context"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/kv/kvpb"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/storage"
	"github.com/cockroachdb/cockroach/pkg/storage/enginepb"
	"github.com/cockroachdb/cockroach/pkg/util/hlc"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/stretchr/testify/require"
)

// TestGCRejectsSpansOutsideRange checks that a GC request planned before a
// split cannot delete data of the right-hand range: a clear range or a range
// key that reaches past the current range is rejected.
func TestGCRejectsSpansOutsideRange(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	ctx := context.Background()
	ts := func(wall int64) hlc.Timestamp { return hlc.Timestamp{WallTime: wall} }

	// The range is [a, m). The keys y1 and y2 belong to the range to the right.
	desc := roachpb.RangeDescriptor{
		RangeID:  1,
		StartKey: roachpb.RKey("a"),
		EndKey:   roachpb.RKey("m"),
	}

	for _, tc := range []struct {
		name string
		req  kvpb.GCRequest
		// points are the keys with a value under the range tombstone.
		points []string
	}{
		{
			name: "clear range",
			req: kvpb.GCRequest{
				ClearRange: &kvpb.GCRequest_GCClearRange{
					StartKey:          roachpb.Key("b"),
					EndKey:            roachpb.Key("z"),
					StartKeyTimestamp: hlc.MaxTimestamp,
				},
			},
			points: []string{"b1", "b2", "y1", "y2"},
		},
		{
			name: "range key",
			req: kvpb.GCRequest{
				RangeKeys: []kvpb.GCRequest_GCRangeKey{
					{StartKey: roachpb.Key("b"), EndKey: roachpb.Key("z"), Timestamp: ts(2)},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			eng := storage.NewDefaultInMemForTesting()
			defer eng.Close()

			// The points have a value at 1 under a range tombstone at 2, so all
			// of it is garbage at the threshold 3.
			for _, k := range tc.points {
				var value roachpb.Value
				value.SetString("v")
				_, err := storage.MVCCPut(ctx, eng, roachpb.Key(k), ts(1), value, storage.MVCCWriteOptions{})
				require.NoError(t, err)
			}
			require.NoError(t, storage.MVCCDeleteRangeUsingTombstone(ctx, eng, nil, /* ms */
				roachpb.Key("b"), roachpb.Key("z"), ts(2), hlc.ClockTimestamp{}, nil, nil, /* peek bounds */
				false /* idempotent */, 0 /* maxLockConflicts */, nil /* msCovered */))

			countRight := func() int {
				iter, err := eng.NewMVCCIterator(storage.MVCCKeyIterKind, storage.IterOptions{
					KeyTypes:   storage.IterKeyTypePointsAndRanges,
					LowerBound: roachpb.Key("m"),
					UpperBound: roachpb.Key("z"),
				})
				require.NoError(t, err)
				defer iter.Close()
				n := 0
				for iter.SeekGE(storage.MVCCKey{Key: roachpb.Key("m")}); ; iter.Next() {
					ok, err := iter.Valid()
					require.NoError(t, err)
					if !ok {
						break
					}
					n++
				}
				return n
			}
			before := countRight()

			req := tc.req
			req.Key, req.EndKey = desc.StartKey.AsRawKey(), desc.EndKey.AsRawKey()
			evalCtx := (&MockEvalCtx{
				ClusterSettings: cluster.MakeTestingClusterSettings(),
				Desc:            &desc,
				GCThreshold:     ts(3),
			}).EvalContext()
			batch := eng.NewBatch()
			defer batch.Close()
			_, err := GC(ctx, batch, CommandArgs{
				EvalCtx: evalCtx,
				Header:  kvpb.Header{Timestamp: ts(4)},
				Args:    &req,
				Stats:   &enginepb.MVCCStats{},
			}, &kvpb.GCResponse{})
			require.ErrorContains(t, err, "is not inside range")
			require.NoError(t, batch.Commit(false /* sync */))
			require.Equal(t, before, countRight())
		})
	}
}
