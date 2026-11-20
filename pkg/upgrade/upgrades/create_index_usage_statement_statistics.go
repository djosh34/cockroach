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

	"github.com/cockroachdb/cockroach/pkg/clusterversion"
	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/systemschema"
	"github.com/cockroachdb/cockroach/pkg/upgrade"
)

const addIndexesUsageCol = `
ALTER TABLE system.statement_statistics
ADD COLUMN IF NOT EXISTS "indexes_usage" JSONB
AS ((statistics->'statistics':::STRING)->'indexes':::STRING) VIRTUAL
`

const addIndexOnIndexesUsageCol = `
CREATE INVERTED INDEX indexes_usage_idx
ON system.statement_statistics (indexes_usage)
`

func createIndexOnIndexUsageOnSystemStatementStatistics(
	ctx context.Context, cs clusterversion.ClusterVersion, d upgrade.TenantDeps,
) error {
	for _, op := range []operation{
		{
			name:           "create-indexes-usage-col-statement-statistics",
			schemaList:     []string{"indexes_usage"},
			query:          addIndexesUsageCol,
			schemaExistsFn: hasColumn,
		},
		{
			name:           "create-indexes-usage-idx-statement-statistics",
			schemaList:     []string{"indexes_usage_idx"},
			query:          addIndexOnIndexesUsageCol,
			schemaExistsFn: hasIndex,
		},
	} {
		if err := migrateTable(ctx, cs, d, op, keys.StatementStatisticsTableID, systemschema.StatementStatisticsTable); err != nil {
			return err
		}
	}
	return nil
}
