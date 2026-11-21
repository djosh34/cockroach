// Copyright 2026 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package backup

import "context"

// FullBackups exports fullBackups to the tests.
var FullBackups = fullBackups

// SegmentNumbers exports segmentNumbers to the tests.
var SegmentNumbers = segmentNumbers

// ReadSegmentHeader reads the header of segment n.
func ReadSegmentHeader(ctx context.Context, s *Store, n int64) (SegmentHeader, error) {
	return readSegment(ctx, s, n, true /* headerOnly */, nil)
}
