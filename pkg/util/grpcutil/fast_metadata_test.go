// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package grpcutil

import (
	"context"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
)

func TestFastFromIncomingContext(t *testing.T) {
	defer leaktest.AfterTest(t)()

	md := metadata.MD{"hello": []string{"world", "universe"}}

	ctx := metadata.NewIncomingContext(context.Background(), md)
	md2, ok := FastFromIncomingContext(ctx)
	require.True(t, ok)
	require.Equal(t, md2, md)

	v, ok := FastFirstValueFromIncomingContext(ctx, "hello")
	require.True(t, ok)
	require.Equal(t, v, "world")
}

func TestClearIncomingContextExcept(t *testing.T) {
	defer leaktest.AfterTest(t)()

	md := metadata.MD{
		"hello":   []string{"world", "universe"},
		"goodbye": []string{"sweet prince", "moon"},
	}

	ctx := metadata.NewIncomingContext(context.Background(), md)

	ctx2 := ClearIncomingContextExcept(ctx, "goodbye")
	md2, ok := FastFromIncomingContext(ctx2)
	require.True(t, ok)

	_, ok = md2["hello"]
	require.False(t, ok)

	_, ok = md2["goodbye"]
	require.True(t, ok)
	require.Equal(t, md["goodbye"], md2["goodbye"])
}

func BenchmarkFromIncomingContext(b *testing.B) {
	md := metadata.MD{"hello": []string{"world", "universe"}}
	ctx := metadata.NewIncomingContext(context.Background(), md)
	b.Run("stdlib", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = metadata.FromIncomingContext(ctx)
		}
	})
	b.Run("fast", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = FastFromIncomingContext(ctx)
		}
	})
}
