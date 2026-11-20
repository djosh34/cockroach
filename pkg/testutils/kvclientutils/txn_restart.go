// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package kvclientutils

import (
	"math/rand"
	"strings"

	"github.com/cockroachdb/cockroach/pkg/kv"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/testutils"
	"github.com/cockroachdb/cockroach/pkg/util/syncutil"
)

func RandomTransactionRetryFilter() func(roachpb.Transaction) bool {
	return func(roachpb.Transaction) bool {
		return rand.Float64() < kv.RandomTxnRetryProbability
	}
}

func PrefixTransactionRetryFilter(
	t testutils.TestErrorer, prefix string, maxCount int,
) (func(roachpb.Transaction) bool, func()) {
	var count int
	var mu syncutil.Mutex
	verifyFunc := func() {
		mu.Lock()
		defer mu.Unlock()
		if count == 0 {
			t.Errorf("expected at least 1 transaction to match prefix %q", prefix)
		}
	}
	filterFunc := func(txn roachpb.Transaction) bool {
		if !strings.HasPrefix(txn.Name, prefix) {
			return false
		}

		mu.Lock()
		defer mu.Unlock()
		if count < maxCount {
			count++
			return true
		}
		return false
	}

	return filterFunc, verifyFunc
}
