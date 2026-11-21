// Copyright 2026 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package upstreamcheck_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/jobs/jobspb"
	"github.com/cockroachdb/cockroach/pkg/security/securityassets"
	"github.com/cockroachdb/cockroach/pkg/security/securitytest"
	"github.com/cockroachdb/cockroach/pkg/server"
	"github.com/cockroachdb/cockroach/pkg/server/upstreamcheck"
	"github.com/cockroachdb/cockroach/pkg/sql"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/sqlutils"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/protoutil"
	"github.com/cockroachdb/cockroach/pkg/util/randutil"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	securityassets.SetLoader(securitytest.EmbeddedAssets)
	randutil.SeedForTests()
	serverutils.InitTestServerFactory(server.TestServerFactory)
	os.Exit(m.Run())
}

// TestCheckJobsAndSchedules writes jobs and a schedule of cut types as
// upstream CockroachDB wrote them, and checks that Check lists the ones that
// still run and that SHOW JOBS lists them all.
func TestCheckJobsAndSchedules(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)
	ctx := context.Background()
	srv, db, _ := serverutils.StartServer(t, base.TestServerArgs{})
	defer srv.Stopper().Stop(ctx)
	sqlDB := sqlutils.MakeSQLRunner(db)
	internalDB := srv.ApplicationLayer().ExecutorConfig().(sql.ExecutorConfig).InternalDB

	// A cluster that patchroaddb made has nothing cut, whatever it holds.
	sqlDB.Exec(t, `CREATE TABLE t (k INT PRIMARY KEY, j JSONB, a INT[], INVERTED INDEX (j), INVERTED INDEX (a))`)
	sqlDB.Exec(t, `CREATE VIEW v AS SELECT k, now() AS n FROM t`)
	sqlDB.Exec(t, `CREATE FUNCTION f(x INT) RETURNS INT LANGUAGE SQL AS 'SELECT x + 1'`)
	require.NoError(t, upstreamcheck.Check(ctx, internalDB))

	// addJob writes a job with the payload that upstream wrote. Paused and
	// finished jobs are not adopted, so the test server leaves them alone.
	addJob := func(id int, status string, details jobspb.Details) {
		payload := jobspb.Payload{Description: "upstream job", Details: jobspb.WrapPayloadDetails(details)}
		if status == "succeeded" {
			payload.FinishedMicros = timeutil.Now().UnixMicro()
		}
		bytes, err := protoutil.Marshal(&payload)
		require.NoError(t, err)
		sqlDB.Exec(t, `INSERT INTO system.jobs (id, status, job_type) VALUES ($1, $2, $3)`,
			id, status, payload.Type().String())
		sqlDB.Exec(t, `INSERT INTO system.job_info (job_id, info_key, value) VALUES ($1, 'legacy_payload', $2)`,
			id, bytes)
	}
	addJob(1, "succeeded", jobspb.BackupDetails{})
	addJob(2, "paused", jobspb.ChangefeedDetails{})
	addJob(3, "failed", jobspb.RestoreDetails{})
	var scheduleID int64
	sqlDB.QueryRow(t, `INSERT INTO system.scheduled_jobs (schedule_name, owner, executor_type, execution_args)
		VALUES ('nightly', 'root', 'scheduled-backup-executor', '') RETURNING schedule_id`).Scan(&scheduleID)

	err := upstreamcheck.Check(ctx, internalDB)
	require.Error(t, err)
	require.Equal(t, fmt.Sprintf(`this cluster uses features that patchroaddb does not have:
  job 2: paused CHANGEFEED job
  schedule %d "nightly": runs scheduled-backup-executor`, scheduleID), err.Error())

	sqlDB.CheckQueryResults(t,
		`SELECT job_id, job_type, status FROM [SHOW JOBS] WHERE job_id IN (1, 2, 3) ORDER BY job_id`,
		[][]string{{"1", "BACKUP", "succeeded"}, {"2", "CHANGEFEED", "paused"}, {"3", "RESTORE", "failed"}})
}
