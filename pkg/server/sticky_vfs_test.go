// Copyright 2019 The Cockroach Authors.
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
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/storage"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/stretchr/testify/require"
)

func TestStickyVFS(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	var (
		ctx       = context.Background()
		attrs     = roachpb.Attributes{}
		cacheSize = int64(1 << 20)   /* 1 MiB */
		storeSize = int64(512 << 20) /* 512 MiB */
		settings  = cluster.MakeTestingClusterSettings()
		registry  = NewStickyVFSRegistry()
	)

	cfg1 := MakeConfig(ctx, settings)
	cfg1.CacheSize = cacheSize
	spec1 := base.StoreSpec{
		StickyVFSID: "engine1",
		Attributes:  attrs,
		Size:        base.SizeSpec{InBytes: storeSize},
	}
	fs1 := registry.Get(spec1.StickyVFSID)
	engine1, err := storage.Open(ctx, storage.MakeLocation("", fs1), settings)
	require.NoError(t, err)
	fs2 := registry.Get(spec1.StickyVFSID)
	require.Equal(t, fs1, fs2)
	require.False(t, engine1.Closed())
	engine1.Close()

	// Refetching the engine should give back a different engine with the same
	// underlying fs.
	fs3 := registry.Get(spec1.StickyVFSID)
	engine2, err := storage.Open(ctx, storage.MakeLocation("", fs3), settings)
	require.NoError(t, err)
	require.NotEqual(t, engine1, engine2)
	require.Equal(t, fs1, fs3)
	require.True(t, engine1.Closed())
	require.False(t, engine2.Closed())
	engine2.Close()

	for _, engine := range []storage.Engine{engine1, engine2} {
		require.True(t, engine.Closed())
	}
}
