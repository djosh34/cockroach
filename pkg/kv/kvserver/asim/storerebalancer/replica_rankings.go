// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package storerebalancer

import (
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/allocator/load"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/asim/state"
)

func hottestRanges(
	state state.State, storeID state.StoreID, dim load.Dimension,
) []kvserver.CandidateReplica {
	replRankings := kvserver.NewReplicaRankings()
	accumulator := kvserver.NewReplicaAccumulator(dim)
	// NB: This follows the actual implementation, where replicas are included
	// regardless of whether the replica is a lease holder. These are later
	// filtered out in the store rebalancer.
	for _, repl := range state.Replicas(storeID) {
		candidateReplica := newSimulatorReplica(repl, state)
		accumulator.AddReplica(candidateReplica)
	}
	replRankings.Update(accumulator)
	return replRankings.TopLoad(dim)
}
