// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package testutils

import (
	"context"
	"strings"

	"github.com/cockroachdb/cockroach/pkg/kv/kvpb"
	"github.com/cockroachdb/cockroach/pkg/util/syncutil"
)

// TestingRequestFilterRetryTxnWithPrefix returns a TestingRequestFilter that
// forces maxCount retry errors for each transaction whose debug name
// begins with the given prefix.
//
// The second return value can be used to verify that at least one
// transaction was retried.
//
// Example:
//
//	filterFunc, verifyFunc := testutils.TestingRequestFilterRetryTxnWithPrefix("ttljob-", 1)
//	base.TestingKnobs{
//		Store: &kvserver.StoreTestingKnobs{
//		 	TestingRequestFilter: filterFunc,
//		},
//	}
func TestingRequestFilterRetryTxnWithPrefix(
	t TestErrorer, prefix string, maxCount int,
) (func(ctx context.Context, r *kvpb.BatchRequest) *kvpb.Error, func()) {
	txnTracker := struct {
		syncutil.Mutex
		retryCounts map[string]int
	}{
		retryCounts: make(map[string]int),
	}
	verifyFunc := func() {
		txnTracker.Lock()
		defer txnTracker.Unlock()
		if len(txnTracker.retryCounts) == 0 {
			t.Errorf("expected at least 1 transaction to match prefix %q", prefix)
		}
	}
	filterFunc := func(ctx context.Context, r *kvpb.BatchRequest) *kvpb.Error {
		if r.Txn == nil || !strings.HasPrefix(r.Txn.Name, prefix) {
			return nil
		}

		if _, ok := r.GetArg(kvpb.EndTxn); ok {
			txnTracker.Lock()
			defer txnTracker.Unlock()
			if txnTracker.retryCounts[r.Txn.Name] < maxCount {
				txnTracker.retryCounts[r.Txn.Name]++
				return kvpb.NewError(kvpb.NewTransactionRetryError(kvpb.RETRY_REASON_UNKNOWN, "injected error"))
			}
		}
		return nil
	}

	return filterFunc, verifyFunc
}
