// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package kvflowdispatch

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/kvflowcontrol"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/kvflowcontrol/kvflowcontrolpb"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
)

type dummy struct{}

var _ kvflowcontrol.Dispatch = &dummy{}

// NewDummyDispatch returns a dummy implementation of kvflowcontrol.Dispatch,
// for use in tests.
func NewDummyDispatch() kvflowcontrol.Dispatch {
	return &dummy{}
}

func (d *dummy) Dispatch(context.Context, roachpb.NodeID, kvflowcontrolpb.AdmittedRaftLogEntries) {
}

func (d *dummy) PendingDispatch() []roachpb.NodeID {
	return nil
}

func (d *dummy) PendingDispatchFor(
	nodeID roachpb.NodeID, maxBytes int64,
) ([]kvflowcontrolpb.AdmittedRaftLogEntries, int) {
	return nil, 0
}
