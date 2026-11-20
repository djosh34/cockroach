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
	"sort"
	"time"

	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/allocator/storepool"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/asim/config"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/util/hlc"
)

// exchangeInfo contains the information of a gossiped store descriptor.
type exchangeInfo struct {
	created time.Time
	desc    roachpb.StoreDescriptor
}

// fixedDelayExchange simulates a gossip exchange network with a symmetric
// fixed delay between all connected clients.
type fixedDelayExchange struct {
	pending  []exchangeInfo
	settings *config.SimulationSettings
}

// put adds the given descriptors at the current tick into the exchange
// network.
func (u *fixedDelayExchange) put(tick time.Time, descs ...roachpb.StoreDescriptor) {
	for _, desc := range descs {
		u.pending = append(u.pending, exchangeInfo{created: tick, desc: desc})
	}
}

// updates returns back exchanged infos, wrapped as store details that have
// completed between the last tick update was called and the tick given.
func (u *fixedDelayExchange) updates(tick time.Time) []*storepool.StoreDetail {
	sort.Slice(u.pending, func(i, j int) bool {
		if u.pending[i].created == u.pending[j].created {
			return i < j
		}
		return u.pending[i].created.Before(u.pending[j].created)
	})
	ready := []*storepool.StoreDetail{}
	i := 0
	for ; i < len(u.pending) && !tick.Before(u.pending[i].created.Add(u.settings.StateExchangeDelay)); i++ {
		ready = append(ready, makeStoreDetail(&u.pending[i].desc, u.pending[i].created))
	}
	u.pending = u.pending[i:]
	return ready
}

// makeStoreDetail wraps a store descriptor into a storepool StoreDetail at the
// given tick.
func makeStoreDetail(desc *roachpb.StoreDescriptor, tick time.Time) *storepool.StoreDetail {
	return &storepool.StoreDetail{
		Desc:            desc,
		LastUpdatedTime: hlc.Timestamp{WallTime: tick.UnixNano()},
	}
}
