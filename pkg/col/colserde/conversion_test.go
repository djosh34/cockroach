// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package colserde_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/apache/arrow/go/arrow/array"
	"github.com/cockroachdb/cockroach/pkg/col/coldata"
	"github.com/cockroachdb/cockroach/pkg/col/colserde"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
	"github.com/stretchr/testify/require"
)

func BenchmarkConversion(b *testing.B) {
	ctx := context.Background()
	createSerializer := func(typ *types.T) (*colserde.ArrowBatchConverter, *colserde.RecordBatchSerializer) {
		c, err := colserde.NewArrowBatchConverter([]*types.T{typ}, colserde.BatchToArrowOnly, testMemAcc)
		require.NoError(b, err)
		s, err := colserde.NewRecordBatchSerializer([]*types.T{typ})
		require.NoError(b, err)
		return c, s
	}
	runConversionBenchmarks(
		b,
		"Serialize",
		func(b *testing.B, batch coldata.Batch, typ *types.T) {
			c, s := createSerializer(typ)
			defer c.Release(ctx)
			var buf bytes.Buffer
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				data, _ := c.BatchToArrow(ctx, batch)
				if len(data) != 1 {
					b.Fatal("expected arrow batch of length 1")
				}
				if data[0].Len() != coldata.BatchSize() {
					b.Fatal("unexpected number of elements")
				}
				buf.Reset()
				_, _, err := s.Serialize(&buf, data, coldata.BatchSize())
				if err != nil {
					b.Fatal(err)
				}
			}
		},
		"Deserialize",
		func(b *testing.B, batch coldata.Batch, typ *types.T) {
			var serialized []byte
			{
				c, s := createSerializer(typ)
				defer c.Release(ctx)
				var buf bytes.Buffer
				data, _ := c.BatchToArrow(ctx, batch)
				if len(data) != 1 {
					b.Fatal("expected arrow batch of length 1")
				}
				if data[0].Len() != coldata.BatchSize() {
					b.Fatal("unexpected number of elements")
				}
				_, _, err := s.Serialize(&buf, data, coldata.BatchSize())
				if err != nil {
					b.Fatal(err)
				}
				serialized = buf.Bytes()
			}
			c, err := colserde.NewArrowBatchConverter([]*types.T{typ}, colserde.ArrowToBatchOnly, nil /* acc */)
			require.NoError(b, err)
			s, err := colserde.NewRecordBatchSerializer([]*types.T{typ})
			require.NoError(b, err)
			result := testAllocator.NewMemBatchWithMaxCapacity([]*types.T{typ})
			var arrowScratch []array.Data
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				arrowScratch = arrowScratch[:0]
				batchLength, err := s.Deserialize(&arrowScratch, serialized)
				if err != nil {
					b.Fatal(err)
				}
				if err = c.ArrowToBatch(arrowScratch, batchLength, result); err != nil {
					b.Fatal(err)
				}
				if result.Width() != 1 {
					b.Fatal("expected one column")
				}
				if result.Length() != coldata.BatchSize() {
					b.Fatal("unexpected number of elements")
				}
			}
		},
	)
}
