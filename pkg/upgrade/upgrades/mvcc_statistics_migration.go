// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

// Package upgrades contains the implementation of upgrades. It is imported
// by the server library.
//
// This package registers the upgrades with the upgrade package.

package upgrades

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/clusterversion"
	"github.com/cockroachdb/cockroach/pkg/jobs"
	"github.com/cockroachdb/cockroach/pkg/jobs/jobspb"
	"github.com/cockroachdb/cockroach/pkg/security/username"
	// Import for the side effect of registering the MVCC statistics update job.
	_ "github.com/cockroachdb/cockroach/pkg/sql"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/systemschema"
	"github.com/cockroachdb/cockroach/pkg/sql/isql"
	"github.com/cockroachdb/cockroach/pkg/upgrade"
)

func createMVCCStatisticsTableAndJobMigration(
	ctx context.Context, _ clusterversion.ClusterVersion, d upgrade.TenantDeps,
) error {

	// Create the table.
	err := createSystemTable(
		ctx,
		d.DB,
		d.Settings,
		d.Codec,
		systemschema.SystemMVCCStatisticsTable,
	)
	if err != nil {
		return err
	}

	// Bake the job.
	if d.TestingKnobs != nil && d.TestingKnobs.SkipMVCCStatisticsJobBootstrap {
		return nil
	}

	record := jobs.Record{
		JobID:         jobs.MVCCStatisticsJobID,
		Description:   "mvcc statistics update job",
		Username:      username.NodeUserName(),
		Details:       jobspb.MVCCStatisticsJobDetails{},
		Progress:      jobspb.MVCCStatisticsJobProgress{},
		NonCancelable: true, // The job can't be canceled, but it can be paused.
	}
	return d.DB.Txn(ctx, func(ctx context.Context, txn isql.Txn) error {
		return d.JobRegistry.CreateIfNotExistAdoptableJobWithTxn(ctx, record, txn)
	})
}
