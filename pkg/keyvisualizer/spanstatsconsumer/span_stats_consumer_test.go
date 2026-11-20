// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package spanstatsconsumer

import (
	"testing"

	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/stretchr/testify/require"
)

func makeBoundaries(n int) []roachpb.Span {
	boundaries := make([]roachpb.Span, n)
	for i := 0; i < n; i++ {
		sp := roachpb.Span{
			Key:    roachpb.Key{byte(i)},
			EndKey: roachpb.Key{byte(i + 1)},
		}
		boundaries[i] = sp
	}
	return boundaries
}

func TestMaybeAggregateBoundaries(t *testing.T) {
	{
		// Case 1: A boundaries slice that does not get modified.
		boundaries := makeBoundaries(10)
		combined, err := maybeAggregateBoundaries(boundaries, 10 /* max */)
		require.NoError(t, err)
		require.Equal(t, len(boundaries), len(combined))
		require.Equal(t, boundaries, combined)
	}
	{
		// Case 2: A boundaries slice where len(boundaries) % 2 == 0.
		boundaries := makeBoundaries(10)
		combined, err := maybeAggregateBoundaries(boundaries, 9 /* max */)
		require.NoError(t, err)
		require.Equal(t, 5, len(combined))
		require.Equal(t, boundaries[0].Combine(boundaries[1]), combined[0])
		require.Equal(t, boundaries[8].Combine(boundaries[9]), combined[4])
	}
	{
		// Case 3: A boundaries slice where len(boundaries) % 2 != 0.
		boundaries := makeBoundaries(33)
		combined, err := maybeAggregateBoundaries(boundaries, 10 /* max */)
		require.NoError(t, err)
		require.Equal(t, 9, len(combined))
		require.Equal(t, boundaries[0].Combine(boundaries[3]), combined[0])
		require.Equal(t, boundaries[32], combined[8])
	}
	{
		// Case 4: A boundaries slice that is completely aggregated.
		boundaries := makeBoundaries(100)
		combined, err := maybeAggregateBoundaries(boundaries, 1 /* max */)
		require.NoError(t, err)
		require.Equal(t, 1, len(combined))
		require.Equal(t, boundaries[0].Combine(boundaries[99]), combined[0])
	}
}
