// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package upgrades

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/clusterversion"
	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/systemschema"
	"github.com/cockroachdb/cockroach/pkg/sql/sessiondata"
	"github.com/cockroachdb/cockroach/pkg/upgrade"
)

const addUserIDColumnToWebSessionsTableStmt = `
ALTER TABLE system.web_sessions
ADD COLUMN IF NOT EXISTS user_id OID
FAMILY "fam_0_id_hashedSecret_username_createdAt_expiresAt_revokedAt_lastUsedAt_auditInfo"
`

func alterWebSessionsTableAddUserIDColumn(
	ctx context.Context, cs clusterversion.ClusterVersion, d upgrade.TenantDeps,
) error {
	for _, op := range []operation{
		{
			name:           "add-user-id-column-web-sessions-table",
			schemaList:     []string{"user_id"},
			query:          addUserIDColumnToWebSessionsTableStmt,
			schemaExistsFn: columnExists,
		},
	} {
		if err := migrateTable(ctx, cs, d, op, keys.WebSessionsTableID, systemschema.WebSessionsTable); err != nil {
			return err
		}
	}

	return nil
}

const backfillUserIDColumnWebSessionsTableStmt = `
UPDATE system.web_sessions AS w
SET user_id = u.user_id
FROM system.users AS u
WHERE w.user_id IS NULL AND w.username = u.username
LIMIT 1000
`

const deleteRowsForDroppedUsersWebSessionsTableStmt = `
DELETE FROM system.web_sessions
WHERE user_id IS NULL
LIMIT 1000
`

const setUserIDColumnToNotNullWebSessionsTableStmt = `
ALTER TABLE system.web_sessions
ALTER COLUMN user_id SET NOT NULL
`

func backfillWebSessionsTableUserIDColumn(
	ctx context.Context, cs clusterversion.ClusterVersion, d upgrade.TenantDeps,
) error {
	ie := d.DB.Executor()
	for _, op := range []struct {
		name  string
		query string
	}{
		{
			name:  "backfill-user-id-col-web-sessions-table",
			query: backfillUserIDColumnWebSessionsTableStmt,
		},
		{
			name:  "delete-rows-for-dropped-users-web-sessions-table",
			query: deleteRowsForDroppedUsersWebSessionsTableStmt,
		},
	} {
		for {
			rowsAffected, err := ie.ExecEx(ctx, op.name, nil /* txn */, sessiondata.NodeUserSessionDataOverride, op.query)
			if err != nil {
				return err
			}
			if rowsAffected == 0 {
				break
			}
		}
	}

	// After we finish backfilling, we can set the user_id column to be NOT NULL
	// since any existing rows will now have non-NULL values in the user_id column
	// and any new rows inserted after the previous version (when the user_id column
	// was added) will have had their user_id value populated at insertion time.
	op := operation{
		name:           "set-user-id-not-null-web-sessions-table",
		schemaList:     []string{"user_id"},
		query:          setUserIDColumnToNotNullWebSessionsTableStmt,
		schemaExistsFn: columnExistsAndIsNotNull,
	}
	if err := migrateTable(ctx, cs, d, op, keys.WebSessionsTableID, systemschema.WebSessionsTable); err != nil {
		return err
	}

	return nil
}
