// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package rangeprober

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/kv"
	"github.com/cockroachdb/cockroach/pkg/kv/kvprober"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
)

// RangeProber implements eval.RangeProber.
type RangeProber struct {
	db  *kv.DB
	ops kvprober.ProberOps
}

// NewRangeProber returns a new instance of a RangeProber.
func NewRangeProber(db *kv.DB) *RangeProber {
	return &RangeProber{
		db: db,
	}
}

// RunProbe implements the eval.RangeProber interface.
func (r *RangeProber) RunProbe(
	ctx context.Context, desc *roachpb.RangeDescriptor, isWrite bool,
) error {
	key := kvprober.ProbeKeyForRange(desc)
	op := r.ops.Read
	if isWrite {
		op = r.ops.Write
	}
	// NB: intentionally using a separate txn per probe to avoid undesirable cross-probe effects.
	return r.db.Txn(ctx, op(key))
}
