// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package decommissioning

import (
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/util/tracing/tracingpb"
)

// RangeCheckResult is the result of evaluating the allocator action
// and target for a single range that has an extant replica on a node targeted
// for decommission.
type RangeCheckResult struct {
	Desc         roachpb.RangeDescriptor
	Action       string
	TracingSpans tracingpb.Recording
	Err          error
}

// PreCheckResult is the result of checking the readiness
// of a node or set of nodes to be decommissioned.
type PreCheckResult struct {
	RangesChecked  int
	ReplicasByNode map[roachpb.NodeID][]roachpb.ReplicaIdent
	ActionCounts   map[string]int
	RangesNotReady []RangeCheckResult
}
