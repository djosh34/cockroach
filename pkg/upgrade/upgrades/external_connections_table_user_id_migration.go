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
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/systemschema"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/sessiondata"
	"github.com/cockroachdb/cockroach/pkg/upgrade"
)

const addOwnerIDColumnToExternalConnectionsTableStmt = `
ALTER TABLE system.external_connections
ADD COLUMN IF NOT EXISTS owner_id OID
FAMILY "primary"
`

func alterExternalConnectionsTableAddOwnerIDColumn(
	ctx context.Context, cs clusterversion.ClusterVersion, d upgrade.TenantDeps,
) error {
	ie := d.DB.Executor()

	// Query the table ID for system.external_connections since it is dynamically
	// assigned.
	idRow, err := ie.QueryRowEx(ctx, "get-table-id", nil, /* txn */
		sessiondata.NodeUserSessionDataOverride,
		`SELECT 'system.external_connections'::regclass::oid`,
	)
	if err != nil {
		return err
	}
	tableID := descpb.ID(tree.MustBeDOid(idRow[0]).Oid)

	for _, op := range []operation{
		{
			name:           "add-owner-id-column-external-connections-table",
			schemaList:     []string{"owner_id"},
			query:          addOwnerIDColumnToExternalConnectionsTableStmt,
			schemaExistsFn: columnExists,
		},
	} {
		if err := migrateTable(ctx, cs, d, op, tableID, systemschema.SystemExternalConnectionsTable); err != nil {
			return err
		}
	}

	return nil
}

const backfillOwnerIDColumnExternalConnectionsTableStmt = `
UPDATE system.external_connections
SET owner_id = user_id
FROM system.users
WHERE owner_id IS NULL AND owner = username
LIMIT 1000
`

const setOwnerIDColumnToNotNullExternalConnectionsTableStmt = `
ALTER TABLE system.external_connections
ALTER COLUMN owner_id SET NOT NULL
`

func backfillExternalConnectionsTableOwnerIDColumn(
	ctx context.Context, cs clusterversion.ClusterVersion, d upgrade.TenantDeps,
) error {
	ie := d.DB.Executor()
	for {
		rowsAffected, err := ie.ExecEx(ctx, "backfill-owner-id-col-external-connections-table", nil, /* txn */
			sessiondata.NodeUserSessionDataOverride,
			backfillOwnerIDColumnExternalConnectionsTableStmt,
		)
		if err != nil {
			return err
		}
		if rowsAffected == 0 {
			break
		}
	}

	// Query the table ID for system.external_connections since it is dynamically
	// assigned.
	idRow, err := ie.QueryRowEx(ctx, "get-table-id", nil, /* txn */
		sessiondata.NodeUserSessionDataOverride,
		`SELECT 'system.external_connections'::regclass::oid`,
	)
	if err != nil {
		return err
	}
	tableID := descpb.ID(tree.MustBeDOid(idRow[0]).Oid)

	// After we finish backfilling, we can set the owner_id column to be NOT NULL
	// since any existing rows will now have non-NULL values in the owner_id column
	// and any new rows inserted after the previous version (when the owner_id column
	// was added) will have had their owner_id value populated at insertion time.
	op := operation{
		name:           "set-owner-id-not-null-external-connections-table",
		schemaList:     []string{"owner_id"},
		query:          setOwnerIDColumnToNotNullExternalConnectionsTableStmt,
		schemaExistsFn: columnExistsAndIsNotNull,
	}
	if err := migrateTable(ctx, cs, d, op, tableID, systemschema.SystemExternalConnectionsTable); err != nil {
		return err
	}

	return nil
}
