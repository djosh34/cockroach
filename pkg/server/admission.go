// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package server

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/kvflowcontrol"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/kvflowcontrol/kvflowcontrolpb"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/util/admission"
	"github.com/cockroachdb/cockroach/pkg/util/admission/admissionpb"
)

type admittedLogEntryAdaptor struct {
	dispatchWriter kvflowcontrol.DispatchWriter
}

var _ admission.OnLogEntryAdmitted = &admittedLogEntryAdaptor{}

func newAdmittedLogEntryAdaptor(
	dispatchWriter kvflowcontrol.DispatchWriter,
) *admittedLogEntryAdaptor {
	return &admittedLogEntryAdaptor{
		dispatchWriter: dispatchWriter,
	}
}

// AdmittedLogEntry implements the admission.OnLogEntryAdmitted interface.
func (a *admittedLogEntryAdaptor) AdmittedLogEntry(
	ctx context.Context,
	origin roachpb.NodeID,
	pri admissionpb.WorkPriority,
	storeID roachpb.StoreID,
	rangeID roachpb.RangeID,
	pos admission.LogPosition,
) {
	// TODO(irfansharif,aaditya): This contributes to a high count of
	// inuse_objects. Look to address it as part of #104154.
	a.dispatchWriter.Dispatch(ctx, origin, kvflowcontrolpb.AdmittedRaftLogEntries{
		RangeID:           rangeID,
		AdmissionPriority: int32(pri),
		UpToRaftLogPosition: kvflowcontrolpb.RaftLogPosition{
			Term:  pos.Term,
			Index: pos.Index,
		},
		StoreID: storeID,
	})
}
