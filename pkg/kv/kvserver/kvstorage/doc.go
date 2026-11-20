// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

// Package kvstorage houses the logic that manages the on-disk state for the
// Replicas housed on a Store. Replicas store data in two storage.Engine
// instances, which are optionally (and at the time of writing, always)
// identical. One Engine, the "log storage", stores Raft-related state (such as
// the raft log), while the other engine stores the replicated keyspace.
//
// The ability to separate log and state machine opens up performance
// improvements, but results in a more complex lifecycle where operations that
// need to update both the state machine and the raft state require more complex
// recovery in the event of an ill-timed crash. Encapsulating and testing this
// logic is the raison d'être for this package.
package kvstorage
