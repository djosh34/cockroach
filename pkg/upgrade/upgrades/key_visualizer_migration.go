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
	"github.com/cockroachdb/cockroach/pkg/keys"
	// Import keyvisjob so that its init function will be called.
	_ "github.com/cockroachdb/cockroach/pkg/keyvisualizer/keyvisjob"
	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/systemschema"
	"github.com/cockroachdb/cockroach/pkg/sql/isql"
	"github.com/cockroachdb/cockroach/pkg/sql/sessiondata"
	"github.com/cockroachdb/cockroach/pkg/upgrade"
)

// keyVisualizerTablesMigration creates the system.span_stats_unique_keys, system.span_stats_buckets,
// system.span_stats_samples, and system.span_stats_tenant_boundaries tables.
func keyVisualizerTablesMigration(
	ctx context.Context, _ clusterversion.ClusterVersion, d upgrade.SystemDeps,
) error {

	tables := []catalog.TableDescriptor{
		systemschema.SpanStatsUniqueKeysTable,
		systemschema.SpanStatsBucketsTable,
		systemschema.SpanStatsSamplesTable,
		systemschema.SpanStatsTenantBoundariesTable,
	}

	for _, table := range tables {
		err := createSystemTable(ctx, d.DB, d.Settings, keys.SystemSQLCodec,
			table)
		if err != nil {
			return err
		}
	}

	// These migrations are skipped for backup tests because these tables are not part
	// of the backup.
	shouldBootstrapJob := true
	shouldConfigureTTL := true

	if knobs := d.KeyVisKnobs; knobs != nil {
		shouldBootstrapJob = !knobs.SkipJobBootstrap
		shouldConfigureTTL = !knobs.SkipZoneConfigBootstrap
	}

	// Set the TTL for the SpanStatsTenantBoundariesTable table.
	if shouldConfigureTTL {
		if _, err := d.DB.Executor().ExecEx(
			ctx,
			"set-SpanStatsTenantBoundariesTable-TTL",
			nil,
			sessiondata.InternalExecutorOverride{User: username.RootUserName()},
			"ALTER TABLE system.span_stats_tenant_boundaries CONFIGURE ZONE USING gc.ttlseconds = $1",
			3600, /* one hour */
		); err != nil {
			return err
		}
	}

	if shouldBootstrapJob {
		record := jobs.Record{
			JobID:         jobs.KeyVisualizerJobID,
			Description:   "key visualizer job",
			Username:      username.NodeUserName(),
			Details:       jobspb.KeyVisualizerDetails{},
			Progress:      jobspb.KeyVisualizerProgress{},
			NonCancelable: true, // The job can't be canceled, but it can be paused.
		}

		return d.DB.Txn(ctx, func(ctx context.Context, txn isql.Txn) error {
			return d.JobRegistry.CreateIfNotExistAdoptableJobWithTxn(ctx, record, txn)
		})
	}

	return nil
}
