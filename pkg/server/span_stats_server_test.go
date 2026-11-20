// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package server

import (
	"context"
	"encoding/binary"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/stretchr/testify/require"
)

func makeSpans(n int, offset int) roachpb.Spans {
	spans := make([]roachpb.Span, 0, n)

	for i := offset; i < n+offset; i++ {
		startKey := make(roachpb.Key, 8)
		endKey := make(roachpb.Key, 8)
		binary.LittleEndian.PutUint64(startKey, uint64(i))
		binary.LittleEndian.PutUint64(endKey, uint64(i+1))
		spans = append(spans, roachpb.Span{Key: startKey, EndKey: endKey})
	}

	return spans
}

func TestSpanStatsBatching(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)
	ctx := context.Background()

	type testCase struct {
		nSpans          int
		limit           int
		expectedBatches [][]roachpb.Span
	}

	testCases := []testCase{
		{nSpans: 2, limit: 5, expectedBatches: [][]roachpb.Span{
			makeSpans(2, 0), // Expect 1 batch of length 2.
		}},
		{nSpans: 5, limit: 5, expectedBatches: [][]roachpb.Span{
			makeSpans(5, 0), // Expect 1 batch of length 5.
		}},
		{nSpans: 4, limit: 3, expectedBatches: [][]roachpb.Span{
			makeSpans(3, 0), // Expect 1st batch of length 3.
			makeSpans(1, 3), // Expect 2nd batch of length 1.
		}},
		{nSpans: 5, limit: 3, expectedBatches: [][]roachpb.Span{
			makeSpans(3, 0), // Expect 1st batch of length 3.
			makeSpans(2, 3), // Expect 2nd batch of length 2.
		}},
		{nSpans: 6, limit: 3, expectedBatches: [][]roachpb.Span{
			makeSpans(3, 0), // Expect 1st batch of length 3.
			makeSpans(3, 3), // Expect 2nd batch of length 3.
		}},
	}

	for _, tcase := range testCases {
		numBatches := 0
		mockSpanStatsEndpoint := func(
			ctx context.Context,
			req *roachpb.SpanStatsRequest,
		) (*roachpb.SpanStatsResponse, error) {

			require.Equal(t, req.Spans, tcase.expectedBatches[numBatches])
			numBatches++

			res := &roachpb.SpanStatsResponse{}
			res.SpanToStats = make(map[string]*roachpb.SpanStats)

			// Provide a response for all spans, and an error per
			// invocation. This lets us confirm that all batched
			// responses make their way back to the caller.
			for _, sp := range req.Spans {
				res.SpanToStats[sp.String()] = &roachpb.SpanStats{}
			}
			res.Errors = append(res.Errors, "some error")
			return res, nil
		}

		req := &roachpb.SpanStatsRequest{}
		req.Spans = makeSpans(tcase.nSpans, 0 /* offset */)
		res, err := batchedSpanStats(ctx, req, mockSpanStatsEndpoint, tcase.limit)
		require.NoError(t, err)

		// Assert that the mocked span stats function is invoked the correct
		// number of times.
		require.Equal(t, len(tcase.expectedBatches), numBatches)

		// Assert that the responses from each batch are returned to the caller.
		require.Equal(t, tcase.nSpans, len(res.SpanToStats))

		// Assert that the errors from each batch are returned to the caller.
		require.Equal(t, len(tcase.expectedBatches), len(res.Errors))
	}

}
