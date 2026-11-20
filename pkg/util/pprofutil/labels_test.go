// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package pprofutil

import (
	"context"
	"runtime/pprof"
	"testing"

	"github.com/cockroachdb/logtags"
	"github.com/stretchr/testify/require"
)

func TestSimple(t *testing.T) {
	ctx := context.Background()
	newCtx, undo := SetProfilerLabels(ctx, "label_key", "label_value")
	defer undo()

	require.NotEqual(t, context.Background(), newCtx)

	v, ok := pprof.Label(ctx, "label_key")
	require.False(t, ok)
	require.Equal(t, "", v)

	v, ok = pprof.Label(newCtx, "label_key")
	require.True(t, ok)
	require.Equal(t, "label_value", v)
}

func TestSimpleFromContext(t *testing.T) {
	ctx := logtags.AddTag(context.Background(), "tag_key", "tag_value")
	newCtx, undo := SetProfilerLabelsFromCtxTags(ctx)
	defer undo()

	require.NotEqual(t, context.Background(), newCtx)

	v, ok := pprof.Label(ctx, "tag_key")
	require.False(t, ok)
	require.Equal(t, "", v)

	v, ok = pprof.Label(newCtx, "tag_key")
	require.True(t, ok)
	require.Equal(t, "tag_value", v)
}

func TestNoTags(t *testing.T) {
	newCtx, undo := SetProfilerLabelsFromCtxTags(context.Background())
	defer undo()
	require.Equal(t, context.Background(), newCtx)
}
