// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package allocatorimpl

import (
	"context"
	"time"

	"github.com/cockroachdb/cockroach/pkg/gossip"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/allocator"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/allocator/storepool"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/liveness"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/liveness/livenesspb"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/util/stop"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
)

// CreateTestAllocator creates a stopper, gossip, store pool and allocator for
// use in tests. Stopper must be stopped by the caller.
func CreateTestAllocator(
	ctx context.Context, numNodes int, deterministic bool,
) (*stop.Stopper, *gossip.Gossip, *storepool.StorePool, Allocator, *timeutil.ManualTime) {
	return CreateTestAllocatorWithKnobs(ctx, numNodes, deterministic, nil /* knobs */)
}

// CreateTestAllocatorWithKnobs is like `CreateTestAllocator`, but allows the
// caller to pass in custom TestingKnobs. Stopper must be stopped by
// the caller.
func CreateTestAllocatorWithKnobs(
	ctx context.Context, numNodes int, deterministic bool, knobs *allocator.TestingKnobs,
) (*stop.Stopper, *gossip.Gossip, *storepool.StorePool, Allocator, *timeutil.ManualTime) {
	st := cluster.MakeTestingClusterSettings()
	stopper, g, manual, storePool, _ := storepool.CreateTestStorePool(ctx, st,
		liveness.TestTimeUntilNodeDeadOff, deterministic,
		func() int { return numNodes },
		livenesspb.NodeLivenessStatus_LIVE)
	a := MakeAllocator(st, deterministic, func(id roachpb.NodeID) (time.Duration, bool) {
		return 0, true
	}, knobs)
	return stopper, g, storePool, a, manual
}
