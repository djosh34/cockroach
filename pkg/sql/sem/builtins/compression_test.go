// Copyright 2026 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package builtins

import (
	"runtime"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/stretchr/testify/require"
)

// TestDecompressIsBounded checks that decompress() refuses output larger than
// 128 MiB, for every codec.
func TestDecompressIsBounded(t *testing.T) {
	defer leaktest.AfterTest(t)()
	large := make([]byte, 128<<20+1)
	for name := range codecs {
		t.Run(name, func(t *testing.T) {
			small, err := compress([]byte("hello"), name)
			require.NoError(t, err)
			out, err := decompress(small, name)
			require.NoError(t, err)
			require.Equal(t, "hello", string(out))

			compressed, err := compress(large, name)
			require.NoError(t, err)
			_, err = decompress(compressed, name)
			require.ErrorContains(t, err, "exceeds the limit")
		})
	}
}

// TestDecompressZstdAllocationIsBounded checks that decompressing a large zstd
// output does not allocate it before the limit applies.
func TestDecompressZstdAllocationIsBounded(t *testing.T) {
	defer leaktest.AfterTest(t)()
	compressed, err := compress(make([]byte, 512<<20), "zstd")
	require.NoError(t, err)

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	_, err = decompress(compressed, "zstd")
	runtime.ReadMemStats(&after)
	require.ErrorContains(t, err, "exceeds the limit")
	require.Less(t, after.TotalAlloc-before.TotalAlloc, uint64(400<<20))
}
