// Copyright 2026 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package backup_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/testutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/sqlutils"
	"github.com/cockroachdb/cockroach/pkg/util/hlc"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/require"
)

// TestRestoreRunningChanges restores to a time when schema changes, an
// IMPORT INTO and an ALTER TYPE were running. The restored cluster must
// finish the schema changes, bring the import's table back with its rows
// from before the import, and keep a materialized view's rows and a TTL
// table's schedule.
func TestRestoreRunningChanges(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)
	ctx := context.Background()
	b := backupSection(t)
	store := openStore(t, b)

	ioDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(ioDir, "rows.csv"), []byte("10\n11\n"), 0600))

	src, db := startServer(t, b, func(args *base.TestServerArgs) { args.ExternalIODir = ioDir })
	defer src.Stopper().Stop(ctx)
	waitFirstFull(t, store)

	db.Exec(t, `CREATE TABLE a (k INT PRIMARY KEY, v INT)`)
	db.Exec(t, `INSERT INTO a SELECT i, i * 10 FROM generate_series(1, 50) AS g(i)`)
	db.Exec(t, `CREATE TABLE b (k INT PRIMARY KEY)`)
	db.Exec(t, `INSERT INTO b SELECT generate_series(1, 20)`)
	db.Exec(t, `CREATE TABLE p (k INT PRIMARY KEY)`)
	db.Exec(t, `INSERT INTO p VALUES (1)`)
	db.Exec(t, `CREATE MATERIALIZED VIEW mv AS SELECT k, v FROM a WHERE k <= 5`)
	db.Exec(t, `CREATE TABLE ttl (k INT PRIMARY KEY) WITH (ttl_expire_after = '10 hours')`)
	db.Exec(t, `CREATE TABLE imp (k INT PRIMARY KEY)`)
	db.Exec(t, `INSERT INTO imp VALUES (1), (2)`)
	db.Exec(t, `CREATE TYPE e AS ENUM ('x')`)

	// Each of these pauses before its job does any work.
	db.Exec(t, `SET CLUSTER SETTING jobs.debug.pausepoints =
		'newschemachanger.before.exec,schemachanger.before.exec,typeschemachanger.before.exec,import.after_ingest'`)
	conn := src.ApplicationLayer().SQLConn(t)
	pausedJobs := func() (n int) {
		db.QueryRow(t, `SELECT count(*) FROM [SHOW JOBS] WHERE status IN ('pause-requested', 'paused')`).Scan(&n)
		return n
	}
	for _, stmt := range []string{
		// A declarative schema change.
		`CREATE INDEX a_v ON a (v)`,
		// A legacy schema change with a backfill.
		`SET use_declarative_schema_changer = off; ALTER TABLE b ADD COLUMN w INT NOT NULL DEFAULT 5`,
		// A table in the ADD state, with a reference from its parent.
		`CREATE TABLE child (k INT PRIMARY KEY, p_k INT REFERENCES p (k))`,
		// A legacy type change.
		`ALTER TYPE e ADD VALUE 'y'`,
		// An import that has written its rows.
		`IMPORT INTO imp CSV DATA ('nodelocal://1/rows.csv')`,
	} {
		// The statement waits for its job, which pauses. Some statements
		// then end with an error, others wait until the server stops.
		stmt := stmt
		before := pausedJobs()
		done := make(chan error, 1)
		go func() { _, err := conn.Exec(stmt); done <- err }()
		testutils.SucceedsSoon(t, func() error {
			select {
			case err := <-done:
				if err == nil || !strings.Contains(err.Error(), "pause") {
					t.Fatalf("%s: %v", stmt, err)
				}
			default:
			}
			if pausedJobs() == before {
				return errors.Newf("waiting for the job of %q to pause", stmt)
			}
			return nil
		})
	}
	db.Exec(t, `RESET CLUSTER SETTING jobs.debug.pausepoints`)
	var now string
	db.QueryRow(t, `SELECT cluster_logical_timestamp()::STRING`).Scan(&now)
	ts, err := hlc.ParseHLC(now)
	require.NoError(t, err)
	waitRestorable(t, store, ts)

	dst, dstDB := startServer(t, nil)
	defer dst.Stopper().Stop(ctx)
	sys := sqlutils.MakeSQLRunner(dst.ApplicationLayer().SQLConn(t, serverutils.DBName("system")))
	sys.Exec(t, `SELECT crdb_internal.restore_cluster($1, $2, $3, $4, $5)`,
		b.Bucket, b.Prefix, b.Endpoint, b.Region, now)

	// The restored jobs finish. GC jobs wait for the GC TTL.
	testutils.SucceedsSoon(t, func() error {
		rows := dstDB.QueryStr(t, `SELECT job_type, status, error FROM [SHOW JOBS]
			WHERE status != 'succeeded' AND job_type != 'SCHEMA CHANGE GC'`)
		if len(rows) > 0 {
			return errors.Newf("jobs not done: %v", rows)
		}
		return nil
	})
	dstDB.CheckQueryResults(t, `SELECT count(*) FROM crdb_internal.invalid_objects`, [][]string{{"0"}})
	dstDB.CheckQueryResults(t, `SELECT count(*), sum(v) FROM a@a_v`, [][]string{{"50", "12750"}})
	dstDB.CheckQueryResults(t, `SELECT count(*) FROM b WHERE w = 5`, [][]string{{"20"}})
	dstDB.Exec(t, `INSERT INTO child VALUES (1, 1)`)
	dstDB.CheckQueryResults(t, `SELECT 'y'::e`, [][]string{{"y"}})
	dstDB.CheckQueryResults(t, `SELECT k FROM imp ORDER BY k`, [][]string{{"1"}, {"2"}})
	dstDB.CheckQueryResults(t, `SELECT k, v FROM mv ORDER BY k`,
		[][]string{{"1", "10"}, {"2", "20"}, {"3", "30"}, {"4", "40"}, {"5", "50"}})
	dstDB.CheckQueryResults(t,
		`SELECT count(*) FROM [SHOW SCHEDULES] WHERE label LIKE 'row-level-ttl%'`, [][]string{{"1"}})
	dstDB.Exec(t, `SELECT crdb_internal.validate_ttl_scheduled_jobs()`)
}
