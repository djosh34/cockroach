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
	"strings"
	"testing"
	"time"

	"github.com/cockroachdb/cockroach/pkg/backup"
	"github.com/cockroachdb/cockroach/pkg/testutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/sqlutils"
	"github.com/cockroachdb/cockroach/pkg/util/hlc"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/require"
)

// TestRestoredClusterDoesNotWriteIntoTheSource restores a cluster into a
// new one that has a copy of the source's config, backup section
// included. The new cluster must refuse to write into the source's
// repository, and nothing about backups is restored.
func TestRestoredClusterDoesNotWriteIntoTheSource(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)
	ctx := context.Background()
	b := backupSection(t)
	store := openStore(t, b)

	src, db := startServer(t, b)
	defer src.Stopper().Stop(ctx)
	var srcID string
	db.QueryRow(t, `SELECT crdb_internal.cluster_id()::STRING`).Scan(&srcID)
	waitFirstFull(t, store)
	db.Exec(t, `CREATE TABLE t (k INT PRIMARY KEY)`)
	db.Exec(t, `INSERT INTO t VALUES (1)`)
	var now string
	db.QueryRow(t, `SELECT cluster_logical_timestamp()::STRING`).Scan(&now)
	ts, err := hlc.ParseHLC(now)
	require.NoError(t, err)
	waitRestorable(t, store, ts)

	// The new cluster has the same backup section and restores the source.
	dst, dstDB := startServer(t, b)
	defer dst.Stopper().Stop(ctx)
	sys := sqlutils.MakeSQLRunner(dst.ApplicationLayer().SQLConn(t, serverutils.DBName("system")))
	sys.Exec(t, `SELECT crdb_internal.restore_cluster($1, $2, $3, $4, $5)`,
		b.Bucket, b.Prefix, b.Endpoint, b.Region, now)
	dstDB.CheckQueryResults(t, `SELECT k FROM defaultdb.t`, [][]string{{"1"}})
	var dstID string
	dstDB.QueryRow(t, `SELECT crdb_internal.cluster_id()::STRING`).Scan(&dstID)
	require.NotEqual(t, srcID, dstID)

	// Its AUTO BACKUP job refuses the repository.
	testutils.SucceedsWithin(t, func() error {
		status := dstDB.QueryStr(t,
			`SELECT running_status FROM crdb_internal.jobs WHERE job_type = 'AUTO BACKUP' AND status = 'running'`)
		if len(status) != 1 || !strings.Contains(status[0][0], "belongs to cluster "+srcID) {
			return errors.Newf("AUTO BACKUP status: %v", status)
		}
		return nil
	}, time.Minute)

	// Every object in the repository was written by the source, while the
	// source keeps backing up.
	fulls, _, err := backup.FullBackups(ctx, store)
	require.NoError(t, err)
	require.NotEmpty(t, fulls)
	for _, m := range fulls {
		require.Equal(t, srcID, m.ClusterID, "full backup %s", m.Name)
	}
	nums, err := backup.SegmentNumbers(ctx, store)
	require.NoError(t, err)
	require.NotEmpty(t, nums)
	for _, n := range nums {
		h, err := backup.ReadSegmentHeader(ctx, store, n)
		require.NoError(t, err)
		require.Equal(t, srcID, h.ClusterID, "segment %d", n)
	}

	// Nothing about backups is restored: no job or schedule came along.
	dstDB.CheckQueryResults(t,
		`SELECT count(*) FROM crdb_internal.jobs WHERE job_type = 'AUTO BACKUP'`, [][]string{{"1"}})
	dstDB.CheckQueryResults(t,
		`SELECT count(*) FROM system.protected_ts_records`, [][]string{{"0"}})
}
