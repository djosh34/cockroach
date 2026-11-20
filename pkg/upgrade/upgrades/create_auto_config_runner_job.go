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
	"github.com/cockroachdb/cockroach/pkg/jobs"
	"github.com/cockroachdb/cockroach/pkg/jobs/jobspb"
	_ "github.com/cockroachdb/cockroach/pkg/jobs/metricspoller" // Ensure job implementation is linked.
	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/cockroach/pkg/sql/isql"
	"github.com/cockroachdb/cockroach/pkg/sql/sessiondata"
	"github.com/cockroachdb/cockroach/pkg/upgrade"
)

func createAutoConfigRunnerJob(
	ctx context.Context, _ clusterversion.ClusterVersion, d upgrade.TenantDeps,
) error {
	if d.TestingKnobs != nil && d.TestingKnobs.SkipAutoConfigRunnerJobBootstrap {
		return nil
	}

	if err := d.DB.Txn(ctx, func(ctx context.Context, txn isql.Txn) error {
		row, err := txn.QueryRowEx(
			ctx,
			"check for existing auto config runner job",
			txn.KV(),
			sessiondata.RootUserSessionDataOverride,
			"SELECT 1 FROM system.jobs WHERE id = $1",
			jobs.AutoConfigRunnerJobID,
		)
		if err != nil {
			return err
		}

		if row != nil {
			// Job already exists. Nothing to do.
			return nil
		}

		jr := jobs.Record{
			JobID:         jobs.AutoConfigRunnerJobID,
			Description:   "applies automatic configuration",
			Details:       jobspb.AutoConfigRunnerDetails{},
			Progress:      jobspb.AutoConfigRunnerProgress{},
			CreatedBy:     &jobs.CreatedByInfo{Name: username.RootUser, ID: username.RootUserID},
			Username:      username.RootUserName(),
			NonCancelable: true,
		}
		// Use CreateJob instead of CreateAdoptableJob to ensure this node
		// has a claim on the job and can start it immediately below.
		_, err = d.JobRegistry.CreateJobWithTxn(ctx, jr, jobs.AutoConfigRunnerJobID, txn)
		return err
	}); err != nil {
		return err
	}

	// Start the job immediately. This speeds up the application
	// of initial configuration tasks.
	d.JobRegistry.NotifyToResume(ctx, jobs.AutoConfigRunnerJobID)
	return nil
}
