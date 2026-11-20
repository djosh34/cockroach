// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package op

import (
	"time"

	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/allocator"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/asim/state"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/errors"
)

// TransferLeaseOp conatins the information for a transfer lease operation.
type TransferLeaseOp struct {
	baseOp
	source, target state.StoreID
	rangeID        state.RangeID
	usage          allocator.RangeUsageInfo
}

// NewTransferLeaseOp returns a new TransferLeaseOp.
func NewTransferLeaseOp(
	tick time.Time,
	rangeID roachpb.RangeID,
	source, target roachpb.StoreID,
	usage allocator.RangeUsageInfo,
) *TransferLeaseOp {
	return &TransferLeaseOp{
		baseOp:  newBaseOp(tick),
		source:  state.StoreID(source),
		target:  state.StoreID(target),
		rangeID: state.RangeID(rangeID),
		usage:   usage,
	}
}

func (tlo *TransferLeaseOp) error(err error) {
	augmentedErr := errors.Wrapf(err, "Unable to transfer lease r=%d source=%d,target=%d",
		tlo.rangeID, tlo.source, tlo.target)
	tlo.errs = append(tlo.errs, augmentedErr)
}
