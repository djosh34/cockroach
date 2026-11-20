// Copyright 2026 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

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
