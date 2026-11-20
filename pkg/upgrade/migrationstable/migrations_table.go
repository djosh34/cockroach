// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package migrationstable

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/kv"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/sql/isql"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/sessiondata"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
)

// MarkMigrationCompleted inserts a row in system.migrations.
func MarkMigrationCompleted(ctx context.Context, ie isql.Executor, v roachpb.Version) error {
	return markMigrationCompletedInner(ctx, ie, v, false /* ignoreExisting */)
}

// MarkMigrationCompletedIdempotent is like MarkMigrationCompleted, except it
// can be called multiple times (or by multiple nodes, concurrently) without
// failing if the respective row already exists. If the row already exists, is
// not changed.
func MarkMigrationCompletedIdempotent(
	ctx context.Context, ie isql.Executor, v roachpb.Version,
) error {
	return markMigrationCompletedInner(ctx, ie, v, true /* ignoreExisting */)
}

func markMigrationCompletedInner(
	ctx context.Context, ie isql.Executor, v roachpb.Version, ignoreExisting bool,
) error {
	query := `
	INSERT
	INTO system.migrations
	(
		major,
		minor,
		patch,
		internal,
		completed_at
	)
	VALUES ($1, $2, $3, $4, $5)
`
	if ignoreExisting {
		query += "ON CONFLICT DO NOTHING"
	}

	_, err := ie.ExecEx(
		ctx,
		"migration-job-mark-job-succeeded",
		nil, /* txn */
		sessiondata.NodeUserSessionDataOverride,
		query,
		v.Major,
		v.Minor,
		v.Patch,
		v.Internal,
		timeutil.Now())
	return err
}

// CheckIfMigrationCompleted queries the system.upgrades table to determine
// if the upgrade associated with this version has already been completed.
// The txn may be nil, in which case the check will be run in its own
// transaction.
func CheckIfMigrationCompleted(
	ctx context.Context, v roachpb.Version, txn *kv.Txn, ex isql.Executor,
) (alreadyCompleted bool, _ error) {
	const query = `
SELECT count(*)
	FROM system.migrations
 WHERE major = $1
	 AND minor = $2
	 AND patch = $3
	 AND internal = $4
`

	row, err := ex.QueryRow(
		ctx,
		"migration-job-find-already-completed",
		txn,
		query,
		v.Major,
		v.Minor,
		v.Patch,
		v.Internal)
	if err != nil {
		return false, err
	}
	count := *row[0].(*tree.DInt)
	return count != 0, nil
}
