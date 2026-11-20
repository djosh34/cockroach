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
	"github.com/cockroachdb/cockroach/pkg/upgrade"
)

func createJobsMetricsPollingJob(
	ctx context.Context, _ clusterversion.ClusterVersion, d upgrade.TenantDeps,
) error {
	if d.TestingKnobs != nil && d.TestingKnobs.SkipJobMetricsPollingJobBootstrap {
		return nil
	}
	return d.DB.Txn(ctx, func(ctx context.Context, txn isql.Txn) error {
		jr := jobs.Record{
			JobID:         jobs.JobMetricsPollerJobID,
			Description:   jobspb.TypePollJobsStats.String(),
			Details:       jobspb.PollJobsStatsDetails{},
			Progress:      jobspb.PollJobsStatsProgress{},
			CreatedBy:     &jobs.CreatedByInfo{Name: username.RootUser, ID: username.RootUserID},
			Username:      username.RootUserName(),
			NonCancelable: true,
		}
		return d.JobRegistry.CreateIfNotExistAdoptableJobWithTxn(ctx, jr, txn)
	})
}
