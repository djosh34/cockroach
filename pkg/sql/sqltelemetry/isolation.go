// Copyright 2024 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package sqltelemetry

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/server/telemetry"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/util/log"
)

// IsolationLevelUpgradedCounter is to be incremented whenever a transaction is
// started with a higher isolation level than the one that the user requested
// for the transaction. It tracks the isolation level that the user requested.
func IsolationLevelUpgradedCounter(
	ctx context.Context, level tree.IsolationLevel,
) telemetry.Counter {
	const prefix = "sql.txn.isolation.upgraded_from."
	var key string
	switch level {
	case tree.SerializableIsolation:
		key = prefix + "serializable"
	case tree.SnapshotIsolation:
		key = prefix + "snapshot"
	case tree.RepeatableReadIsolation:
		key = prefix + "repeatable_read"
	case tree.ReadCommittedIsolation:
		key = prefix + "read_committed"
	case tree.ReadUncommittedIsolation:
		key = prefix + "read_uncommitted"
	default:
		log.Warningf(ctx, "unexpected isolation level: %v", level)
	}
	return telemetry.GetCounter(key)
}
