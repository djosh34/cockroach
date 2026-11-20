// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package state

import (
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
)

// CapacityChangeListener listens for notification of capacity change events.
type CapacityChangeListener interface {
	// CapacityChangeNotify notifies that a capacity change event has occurred
	// for the store with ID StoreID.
	CapacityChangeNotify(kvserver.CapacityChangeEvent, StoreID)
}

// NewCapacityListener listens for notification of new store capacity events.
type NewCapacityListener interface {
	// NewCapacityNotify notifies that a new capacity event has occurred for
	// the store with ID StoreID.
	NewCapacityNotify(roachpb.StoreCapacity, StoreID)
}

// ConfigChangeListener listens for notification of configuration changes such
// as stores being added.
type ConfigChangeListener interface {
	// StoreAddNotify notifies that a new store has been added with ID storeID.
	StoreAddNotify(StoreID, State)
}
