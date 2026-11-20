// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package loqrecovery

import (
	"context"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/clusterversion"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/kvstorage"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/storage"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/stretchr/testify/require"
)

func TestCollectChecksStoreInfo(t *testing.T) {
	leaktest.AfterTest(t)
	ctx := context.Background()

	eng, err := storage.Open(ctx,
		storage.InMemory(),
		cluster.MakeClusterSettings(),
		storage.CacheSize(1<<20 /* 1 MiB */))
	require.NoError(t, err, "failed to create engine")
	defer eng.Close()
	v := roachpb.Version{Major: 21, Minor: 2}
	if err := kvstorage.WriteClusterVersionToEngines(ctx, []storage.Engine{eng},
		clusterversion.ClusterVersion{Version: v}); err != nil {
		t.Fatalf("failed to populate test store cluster version: %v", err)
	}

	_, _, err = CollectStoresReplicaInfo(ctx, []storage.Engine{eng})
	require.ErrorContains(t, err, "is too old for running version",
		"engine version check not triggered")
}
