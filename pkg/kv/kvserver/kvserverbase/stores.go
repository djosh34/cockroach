// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package kvserverbase

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/util/errorutil"
	"github.com/cockroachdb/cockroach/pkg/util/syncutil"
	"github.com/cockroachdb/cockroach/pkg/util/tracing/tracingpb"
)

// StoresIterator is able to iterate over all stores on a given node.
type StoresIterator interface {
	ForEachStore(func(Store) error) error
}

// Store is an adapter to the underlying KV store.
type Store interface {
	// StoreID returns the store identifier.
	StoreID() roachpb.StoreID

	// Enqueue the replica with the given range ID into the named queue.
	Enqueue(
		ctx context.Context,
		queue string,
		rangeID roachpb.RangeID,
		skipShouldQueue bool,
	) (tracingpb.Recording, error)

	// SetQueueActive disables/enables the named queue.
	SetQueueActive(active bool, queue string) error

	// GetReplicaMutexForTesting returns the mutex of the replica with the given
	// range ID, or nil if no replica was found. This is used for testing.
	GetReplicaMutexForTesting(rangeID roachpb.RangeID) *syncutil.RWMutex
}

// UnsupportedStoresIterator is a StoresIterator that only returns "unsupported"
// errors.
type UnsupportedStoresIterator struct{}

var _ StoresIterator = UnsupportedStoresIterator{}

// ForEachStore is part of the StoresIterator interface.
func (i UnsupportedStoresIterator) ForEachStore(f func(Store) error) error {
	return errorutil.UnsupportedUnderClusterVirtualization(errorutil.FeatureNotAvailableToNonSystemTenantsIssue)
}
