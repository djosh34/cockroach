// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package batcheval

import "github.com/cockroachdb/cockroach/pkg/roachpb"

// rangeTombstonePeekBounds returns the left and right bounds that
// MVCCDeleteRangeUsingTombstone can read in order to detect adjacent range
// tombstones to merge with or fragment. The bounds will be truncated to the
// Raft range bounds if given.
func rangeTombstonePeekBounds(
	startKey, endKey, rangeStart, rangeEnd roachpb.Key,
) (roachpb.Key, roachpb.Key) {
	leftPeekBound := startKey.Prevish(roachpb.PrevishKeyLength)
	if len(rangeStart) > 0 && leftPeekBound.Compare(rangeStart) <= 0 {
		leftPeekBound = rangeStart
	}

	rightPeekBound := endKey.Next()
	if len(rangeEnd) > 0 && rightPeekBound.Compare(rangeEnd) >= 0 {
		rightPeekBound = rangeEnd
	}

	return leftPeekBound.Clone(), rightPeekBound.Clone()
}
