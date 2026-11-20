// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package load

import "github.com/cockroachdb/cockroach/pkg/roachpb"

// RecordBatchRequests records the value for number of batch requests at the
// current time against the gateway nodeID.
func (rl *ReplicaLoad) RecordBatchRequests(val float64, nodeID roachpb.NodeID) {
	rl.record(Queries, val, nodeID)
}

// RecordRequests records the value given for requests.
func (rl *ReplicaLoad) RecordRequests(val float64) {
	rl.record(Requests, val, 0 /* nodeID */)
}

// RecordWriteKeys records the value given for write keys.
func (rl *ReplicaLoad) RecordWriteKeys(val float64) {
	rl.record(WriteKeys, val, 0 /* nodeID */)
}

// RecordReadKeys records the value given for read keys.
func (rl *ReplicaLoad) RecordReadKeys(val float64) {
	rl.record(ReadKeys, val, 0 /* nodeID */)
}

// RecordWriteBytes records the value given for write bytes.
func (rl *ReplicaLoad) RecordWriteBytes(val float64) {
	rl.record(WriteBytes, val, 0 /* nodeID */)
}

// RecordReadBytes records the value given for read bytes.
func (rl *ReplicaLoad) RecordReadBytes(val float64) {
	rl.record(ReadBytes, val, 0 /* nodeID */)
}

// RecordRaftCPUNanos records the value given for raft cpu nanos.
func (rl *ReplicaLoad) RecordRaftCPUNanos(val float64) {
	rl.record(RaftCPUNanos, val, 0 /* nodeID */)
}

// RecordReqCPUNanos records the value given for request cpu nanos.
func (rl *ReplicaLoad) RecordReqCPUNanos(val float64) {
	rl.record(ReqCPUNanos, val, 0 /* nodeID */)
}
