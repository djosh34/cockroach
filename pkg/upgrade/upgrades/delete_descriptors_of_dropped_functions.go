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
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descs"
	"github.com/cockroachdb/cockroach/pkg/sql/isql"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/upgrade"
)

const getMaxIDInBatch = `
WITH id_batch AS (
    SELECT id
    FROM system.descriptor
    WHERE id > $1
    ORDER BY id
    LIMIT $2
)
SELECT max(id) from id_batch;
`

const deleteDroppedFunctionQuery = `
WITH to_json AS (
    SELECT
      id,
      crdb_internal.pb_to_json(
        'cockroach.sql.sqlbase.Descriptor',
        descriptor,
        false
      ) AS d
    FROM
      system.descriptor
    WHERE id > $1
    AND id <= $2
),
to_delete AS (
    SELECT id
    FROM to_json
    WHERE
      d->'function' IS NOT NULL
      AND d->'function'->>'declarativeSchemaChangerState' IS NULL
      AND d->'function'->>'state' = 'DROP'
)
SELECT crdb_internal.unsafe_delete_descriptor(id, false)
FROM to_delete;
`

func deleteDescriptorsOfDroppedFunctions(
	ctx context.Context, _ clusterversion.ClusterVersion, d upgrade.TenantDeps,
) error {
	var maxID int64
	if err := d.DB.Txn(ctx, func(ctx context.Context, txn isql.Txn) error {
		row, err := txn.QueryRow(
			ctx,
			"upgrade-delete-dropped-function-descriptors-get-max-descriptor-id",
			txn.KV(),
			`SELECT max(id) FROM system.descriptor`,
		)

		if err != nil {
			return err
		}

		maxID = int64(tree.MustBeDInt(row[0]))
		return nil
	}); err != nil {
		return err
	}

	const batchSize = 50
	for curID := int64(0); curID < maxID; {
		var batchMaxID int64
		if err := d.DB.DescsTxn(ctx, func(ctx context.Context, txn descs.Txn) error {
			row, err := txn.QueryRow(
				ctx,
				"upgrade-delete-dropped-function-descriptors-get-batch-max-id",
				txn.KV(),
				getMaxIDInBatch,
				curID,
				batchSize,
			)
			if err != nil {
				return err
			}
			batchMaxID = int64(tree.MustBeDInt(row[0]))
			_, err = txn.Exec(
				ctx,
				"upgrade-delete-dropped-function-descriptors", /* opName */
				txn.KV(),
				deleteDroppedFunctionQuery,
				curID,
				batchMaxID,
			)
			return err
		}); err != nil {
			return err
		}
		curID = batchMaxID
	}
	return nil
}
