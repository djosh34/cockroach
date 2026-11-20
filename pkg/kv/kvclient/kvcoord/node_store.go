// Copyright 2020 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package kvcoord

import "github.com/cockroachdb/cockroach/pkg/roachpb"

// NodeDescStore stores a collection of NodeDescriptors and StoreDescriptors.
//
// Implementations of the interface are expected to be threadsafe.
type NodeDescStore interface {
	// GetNodeDescriptor looks up the node descriptor by node ID.
	// It returns an error if the node is not known by the cache.
	GetNodeDescriptor(roachpb.NodeID) (*roachpb.NodeDescriptor, error)
	// GetNodeDescriptorCount get the number of node descriptors in the cache.
	// TODO(ewall): Replace this function with a new KV endpoint for 23.1.
	//  See https://github.com/cockroachdb/cockroach/issues/93549.
	GetNodeDescriptorCount() int
	// GetStoreDescriptor looks up the store descriptor by store ID.
	// It returns an error if the store is not known by the cache.
	GetStoreDescriptor(roachpb.StoreID) (*roachpb.StoreDescriptor, error)
}
