// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package gossip

import (
	"testing"

	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/asim/config"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/stretchr/testify/require"
)

func TestFixedDelayExchange(t *testing.T) {
	makeStoresFn := func(stores []int32) []roachpb.StoreDescriptor {
		descriptors := make([]roachpb.StoreDescriptor, len(stores))
		for i := range stores {
			descriptors[i] = roachpb.StoreDescriptor{StoreID: roachpb.StoreID(stores[i])}

		}
		return descriptors
	}

	settings := config.DefaultSimulationSettings()
	tick := settings.StartTime
	exchange := fixedDelayExchange{pending: []exchangeInfo{}, settings: settings}

	// There should be no updates initially.
	require.Len(t, exchange.updates(tick), 0)

	// Put an update at the current tick.
	exchange.put(tick, makeStoresFn([]int32{1, 2, 3})...)
	require.Len(t, exchange.pending, 3)

	// There should be no updates until after the tick + state exchange delay.
	halfTick := tick.Add(settings.StateExchangeDelay / 2)
	require.Len(t, exchange.updates(halfTick), 0)

	// Update the tick to be >= tick + delay, there should be three updates.
	tick = tick.Add(settings.StateExchangeDelay)
	require.Len(t, exchange.updates(tick), 3)
	require.Len(t, exchange.pending, 0)
}
