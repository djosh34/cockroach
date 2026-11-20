// Copyright 2022 The Cockroach Authors.
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
	"fmt"

	"github.com/cockroachdb/cockroach/pkg/clusterversion"
	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/systemschema"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/sessiondata"
	"github.com/cockroachdb/cockroach/pkg/upgrade"
)

const addUserIDColumnToSystemPrivilegesStmt = `
ALTER TABLE system.privileges
ADD COLUMN IF NOT EXISTS user_id OID
FAMILY "primary"
`

const createUniqueIndexOnUserIDAndPathOnSystemPrivilegesStmt = `
CREATE UNIQUE INDEX IF NOT EXISTS privileges_path_user_id_key ON system.privileges (path, user_id) STORING (privileges, grant_options)
`

func alterSystemPrivilegesAddUserIDColumn(
	ctx context.Context, cs clusterversion.ClusterVersion, d upgrade.TenantDeps,
) error {
	ie := d.DB.Executor()

	// Query the table ID for the system.privileges table since it is dynamically
	// assigned.
	idRow, err := ie.QueryRowEx(ctx, "get-table-id", nil, /* txn */
		sessiondata.NodeUserSessionDataOverride,
		`SELECT 'system.privileges'::regclass::oid`,
	)
	if err != nil {
		return err
	}
	tableID := descpb.ID(tree.MustBeDOid(idRow[0]).Oid)

	for _, op := range []operation{
		{
			name:           "add-user-id-column-system-privileges",
			schemaList:     []string{"user_id"},
			query:          addUserIDColumnToSystemPrivilegesStmt,
			schemaExistsFn: columnExists,
		},
		{
			name:           "add-user-id-path-unique-index-system-privileges",
			schemaList:     []string{"privileges_path_user_id_key"},
			query:          createUniqueIndexOnUserIDAndPathOnSystemPrivilegesStmt,
			schemaExistsFn: hasIndex,
		},
	} {
		if err := migrateTable(ctx, cs, d, op, tableID, systemschema.SystemPrivilegeTable); err != nil {
			return err
		}
	}

	return nil
}

var backfillUserIDColumnSystemPrivilegesStmt = fmt.Sprintf(`
UPDATE system.privileges AS p
SET user_id = (
	SELECT CASE p.username
		WHEN '%s' THEN %d
		ELSE (SELECT user_id FROM system.users AS u WHERE u.username = p.username)
	END
)
WHERE p.user_id IS NULL
LIMIT 1000
`,
	username.PublicRole, username.PublicRoleID)

const setUserIDColumnToNotNullSystemPrivilegesStmt = `
ALTER TABLE system.privileges
ALTER COLUMN user_id SET NOT NULL
`

func backfillSystemPrivilegesUserIDColumn(
	ctx context.Context, cs clusterversion.ClusterVersion, d upgrade.TenantDeps,
) error {
	ie := d.DB.Executor()
	for {
		rowsAffected, err := ie.ExecEx(ctx, "backfill-user-id-column-system-privileges", nil, /* txn */
			sessiondata.NodeUserSessionDataOverride,
			backfillUserIDColumnSystemPrivilegesStmt,
		)
		if err != nil {
			return err
		}
		if rowsAffected == 0 {
			break
		}
	}

	// Query the table ID for the system.privileges table since it is dynamically
	// assigned.
	idRow, err := ie.QueryRowEx(ctx, "get-table-id", nil, /* txn */
		sessiondata.NodeUserSessionDataOverride,
		`SELECT 'system.privileges'::regclass::oid`,
	)
	if err != nil {
		return err
	}
	tableID := descpb.ID(tree.MustBeDOid(idRow[0]).Oid)

	// After we finish backfilling, we can set the user_id column to be NOT NULL
	// since any existing rows will now have non-NULL values in the user_id column
	// and any new rows inserted after the previous version (when the user_id column
	// was added) will have had their user_id value populated at insertion time.
	op := operation{
		name:           "set-user-id-not-null-system-privileges",
		schemaList:     []string{"user_id"},
		query:          setUserIDColumnToNotNullSystemPrivilegesStmt,
		schemaExistsFn: columnExistsAndIsNotNull,
	}
	if err := migrateTable(ctx, cs, d, op, tableID, systemschema.SystemPrivilegeTable); err != nil {
		return err
	}

	return nil
}
