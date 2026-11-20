// Copyright 2014 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package result

import "github.com/cockroachdb/cockroach/pkg/roachpb"

// WithAcquiredLocks creates a Result communicating that the supplied lock
// acquisitions (or re-acquisitions) were performed by the caller and they
// should be handled.
//
// If any empty lock acquisitions are supplied by the caller they will not be
// attached to the returned Result.
func WithAcquiredLocks(acqs ...roachpb.LockAcquisition) Result {
	var pd Result
	numAcqs := 0
	for _, acq := range acqs {
		if !acq.Empty() {
			numAcqs++
		}
	}
	if numAcqs == 0 { // only allocate if there is at least 1 non-empty acquisition
		return pd
	}
	pd.Local.AcquiredLocks = make([]roachpb.LockAcquisition, 0, numAcqs)
	for _, acq := range acqs {
		if !acq.Empty() {
			pd.Local.AcquiredLocks = append(pd.Local.AcquiredLocks, acq)
		}
	}
	return pd
}

// EndTxnIntents contains a finished transaction and a bool (Always),
// which indicates whether the intents should be resolved whether or
// not the command succeeds through Raft.
type EndTxnIntents struct {
	Txn    *roachpb.Transaction
	Always bool
	Poison bool
}

// FromEndTxn creates a Result communicating that a transaction was
// completed and its locks should be resolved.
func FromEndTxn(txn *roachpb.Transaction, alwaysReturn, poison bool) Result {
	var pd Result
	if len(txn.LockSpans) == 0 {
		return pd
	}
	pd.Local.EndTxns = []EndTxnIntents{{Txn: txn, Always: alwaysReturn, Poison: poison}}
	return pd
}
