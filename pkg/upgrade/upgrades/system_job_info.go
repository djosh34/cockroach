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
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descs"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/systemschema"
	"github.com/cockroachdb/cockroach/pkg/sql/sessiondata"
	"github.com/cockroachdb/cockroach/pkg/upgrade"
)

// systemPrivilegesTableMigration creates the system.job_info table.
func systemJobInfoTableMigration(
	ctx context.Context, _ clusterversion.ClusterVersion, d upgrade.TenantDeps,
) error {
	// Create the job_info table proper.
	if err := createSystemTable(
		ctx, d.DB, d.Settings, d.Codec, systemschema.SystemJobInfoTable,
	); err != nil {
		return err
	}

	// Also bump the version of the descriptor for system.jobs. This
	// ensures that this migration synchronizes with all other
	// migrations that touch jobs, and concurrent transactions that
	// perform DDL operations. We need this because just creating a new
	// table above does not synchronize with other accesses to
	// system.jobs on its own.
	return d.DB.DescsTxn(ctx, func(
		ctx context.Context, txn descs.Txn,
	) (err error) {
		collection := txn.Descriptors()
		t, err := collection.MutableByID(txn.KV()).Desc(ctx, keys.JobsTableID)
		if err != nil {
			return err
		}
		return collection.WriteDesc(ctx, true /* kvTracing */, t, txn.KV())
	})
}

const alterPayloadToNullableQuery = `
ALTER TABLE system.jobs ALTER COLUMN payload DROP NOT NULL
`

// alterPayloadColumnToNullable runs a schema change to drop the NOT NULL
// constraint on the system.jobs payload column.
func alterPayloadColumnToNullable(
	ctx context.Context, _ clusterversion.ClusterVersion, d upgrade.TenantDeps,
) error {
	_, err := d.InternalExecutor.ExecEx(ctx, "set-job-payload-nullable", nil,
		sessiondata.NodeUserSessionDataOverride, alterPayloadToNullableQuery)
	return err
}
