// Copyright 2024 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

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
