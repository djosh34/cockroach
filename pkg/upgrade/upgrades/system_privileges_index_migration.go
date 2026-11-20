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

func alterSystemPrivilegesAddSecondaryIndex(
	ctx context.Context, cs clusterversion.ClusterVersion, d upgrade.TenantDeps,
) error {
	ie := d.DB.Executor()

	// Query the table ID for the system.privileges table since it is dynamically
	idRow, err := ie.QueryRowEx(ctx, "get-table-id", nil, /* txn */
		sessiondata.NodeUserSessionDataOverride,
		`SELECT 'system.privileges'::regclass::oid`,
	)
	if err != nil {
		return err
	}

	op := operation{
		name:           "add-secondary-index-system-priv",
		schemaList:     []string{"privileges_path_username_key"},
		query:          `CREATE UNIQUE INDEX IF NOT EXISTS privileges_path_username_key ON system.privileges (path,username) STORING (privileges, grant_options)`,
		schemaExistsFn: hasIndex,
	}

	tableID := descpb.ID(tree.MustBeDOid(idRow[0]).Oid)
	if err := migrateTable(ctx, cs, d, op, tableID, systemschema.SystemPrivilegeTable); err != nil {
		return err
	}

	return nil
}
