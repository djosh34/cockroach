// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package upgrades_test

import (
	"context"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/testutils/testcluster"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCreateJobsMetricsPollingJob is testing the permanent upgrade associated
// with Permanent_V23_1_CreateJobsMetricsPollingJob. We no longer support
// versions this old, but we still need to test that the upgrade happens as
// expected when creating a new cluster.
func TestCreateJobsMetricsPollingJob(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	var (
		ctx   = context.Background()
		tc    = testcluster.StartTestCluster(t, 1, base.TestClusterArgs{})
		sqlDB = tc.ServerConn(0)
	)
	defer tc.Stopper().Stop(ctx)

	row := sqlDB.QueryRow("SELECT count(*) FROM crdb_internal.jobs WHERE job_type = 'POLL JOBS STATS'")
	var count int
	err := row.Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, count, 1)
}
