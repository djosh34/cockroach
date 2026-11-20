// Copyright 2020 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package kvserver

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/kv/kvpb"
	"github.com/cockroachdb/cockroach/pkg/multitenant/tenantcostmodel"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/util/quotapool"
	"github.com/cockroachdb/errors"
)

// maybeRateLimitBatch may block the batch waiting to be rate-limited. Note that
// the replica must be initialized and thus there is no synchronization issue
// on the tenantRateLimiter.
func (r *Replica) maybeRateLimitBatch(ctx context.Context, ba *kvpb.BatchRequest) error {
	if r.tenantLimiter == nil {
		return nil
	}
	tenantID, ok := roachpb.ClientTenantFromContext(ctx)
	if !ok || tenantID == roachpb.SystemTenantID {
		return nil
	}

	// writeMultiplier isn't needed here since it's only used to calculate RUs.
	err := r.tenantLimiter.Wait(ctx, tenantcostmodel.MakeRequestInfo(ba, 1, 1))

	// For performance reasons, we do not hold any Replica's mutexes while waiting
	// on the tenantLimiter, and so we are racing with the Replica lifecycle. The
	// Replica can be destroyed and release the limiter before or during the Wait
	// call. In this case Wait returns an ErrClosed error. Instead of ErrClosed,
	// return the destruction status error which upper layers recognize.
	if err != nil && errors.HasType(err, (*quotapool.ErrClosed)(nil)) {
		if _, err := r.IsDestroyed(); err != nil {
			return err
		}
		return errors.AssertionFailedf("replica not marked as destroyed but limiter is closed: %v", r)
	}

	return err
}

// recordImpactOnRateLimiter is used to record a read against the tenant rate
// limiter.
func (r *Replica) recordImpactOnRateLimiter(
	ctx context.Context, br *kvpb.BatchResponse, isReadOnly bool,
) {
	if r.tenantLimiter == nil || br == nil || !isReadOnly {
		return
	}
	// readMultiplier isn't needed here since it's only used to calculate RUs.
	r.tenantLimiter.RecordRead(ctx, tenantcostmodel.MakeResponseInfo(br, isReadOnly, 1))
}
