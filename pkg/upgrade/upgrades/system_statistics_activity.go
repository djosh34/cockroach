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
	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/systemschema"
	"github.com/cockroachdb/cockroach/pkg/upgrade"
)

// systemStatementStatisticsActivityTableMigration creates the
// system.statement_activity and system.transaction_activity table.
func systemStatisticsActivityTableMigration(
	ctx context.Context, _ clusterversion.ClusterVersion, d upgrade.TenantDeps,
) error {

	tables := []catalog.TableDescriptor{
		systemschema.StatementActivityTable,
		systemschema.TransactionActivityTable,
	}

	for _, table := range tables {
		err := createSystemTable(ctx, d.DB, d.Settings, d.Codec,
			table)
		if err != nil {
			return err
		}
	}

	return nil
}
