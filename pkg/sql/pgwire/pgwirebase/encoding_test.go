// Copyright 2026 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package pgwirebase

import (
	"context"
	"encoding/binary"
	"runtime"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/stretchr/testify/require"
)

// TestDecodeBinaryTupleCountIsBounded checks that a tuple header claiming many
// elements in a short message fails without allocating for that count.
func TestDecodeBinaryTupleCountIsBounded(t *testing.T) {
	defer leaktest.AfterTest(t)()
	b := binary.BigEndian.AppendUint32(nil, 1<<22)
	b = append(b, make([]byte, 8)...)

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, err := decodeBinaryTuple(context.Background(), nil /* evalCtx */, b)
	runtime.ReadMemStats(&after)

	require.ErrorContains(t, err, "more elements than")
	require.Less(t, after.TotalAlloc-before.TotalAlloc, uint64(1<<20))
}
