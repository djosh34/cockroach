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
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cockroachdb/cockroach/pkg/backup"
	"github.com/cockroachdb/cockroach/pkg/backup/backuptest"
	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/security/certconfig"
	"github.com/cockroachdb/cockroach/pkg/testutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/sqlutils"
	"github.com/cockroachdb/cockroach/pkg/util/hlc"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/require"
)

// backupSection returns a backup section for a new bucket on the test S3
// server. Full backups are taken every few seconds.
func backupSection(t *testing.T) *certconfig.Backup {
	bucket, endpoint := backuptest.NewBucket(t)
	return &certconfig.Backup{
		Bucket:          bucket,
		Prefix:          "cluster",
		Endpoint:        endpoint,
		Region:          backuptest.Region,
		FullSchedule:    "@every 3s",
		ArchiveInterval: 200 * time.Millisecond,
		KeepFull:        100,
	}
}

func startServer(
	t *testing.T, b *certconfig.Backup, opts ...func(*base.TestServerArgs),
) (serverutils.TestServerInterface, *sqlutils.SQLRunner) {
	certs := certconfig.Embedded()
	certs.Backup = b
	args := base.TestServerArgs{
		Certs:             certs,
		DefaultTestTenant: base.TestIsSpecificToStorageLayerAndNeedsASystemTenant,
	}
	for _, o := range opts {
		o(&args)
	}
	s, db, _ := serverutils.StartServer(t, args)
	return s, sqlutils.MakeSQLRunner(db)
}

func openStore(t *testing.T, b *certconfig.Backup) *backup.Store {
	s, err := backup.OpenStore(context.Background(), backup.RepositoryOf(b))
	require.NoError(t, err)
	return s
}

// waitFirstFull waits for the first full backup, which the AUTO BACKUP job
// takes when it starts.
func waitFirstFull(t *testing.T, s *backup.Store) {
	testutils.SucceedsWithin(t, func() error {
		fulls, _, err := backup.FullBackups(context.Background(), s)
		if err == nil && len(fulls) == 0 {
			err = errors.New("no full backup yet")
		}
		return err
	}, time.Minute)
}

// waitRestorable waits until the repository can restore to ts.
func waitRestorable(t *testing.T, s *backup.Store, ts hlc.Timestamp) {
	testutils.SucceedsWithin(t, func() error {
		_, err := backup.CheckTarget(context.Background(), s, ts)
		return err
	}, time.Minute)
}

// dump returns what a user sees of the cluster: databases, schemas, types,
// tables with their rows, sequences, users, roles, grants and a setting.
func dump(t *testing.T, db *sqlutils.SQLRunner) []string {
	var out []string
	add := func(prefix string, rows [][]string) {
		for _, r := range rows {
			out = append(out, prefix+": "+strings.Join(r, " | "))
		}
	}
	add("database", db.QueryStr(t, `SELECT database_name, owner FROM [SHOW DATABASES] ORDER BY 1`))
	add("user", db.QueryStr(t, `SELECT username, options, member_of::STRING FROM [SHOW USERS] ORDER BY 1`))
	add("setting", db.QueryStr(t, `SHOW CLUSTER SETTING sql.defaults.default_int_size`))
	add("system grant", db.QueryStr(t, `SELECT username, path, privileges::STRING FROM system.privileges ORDER BY 1, 2`))
	for _, r := range db.QueryStr(t, `SELECT database_name FROM [SHOW DATABASES] WHERE database_name != 'system' ORDER BY 1`) {
		name := r[0]
		add("schema "+name, db.QueryStr(t, fmt.Sprintf(`SELECT schema_name, owner FROM [SHOW SCHEMAS FROM %q] ORDER BY 1`, name)))
		add("type "+name, db.QueryStr(t, fmt.Sprintf(
			`SELECT create_statement FROM %q.crdb_internal.create_type_statements ORDER BY 1`, name)))
		add("create "+name, db.QueryStr(t, fmt.Sprintf(
			`SELECT create_statement FROM %q.crdb_internal.create_statements
			 WHERE database_name = %s AND schema_name NOT IN ('pg_catalog', 'information_schema', 'crdb_internal', 'pg_extension')
			 ORDER BY schema_name, descriptor_name`, name, lexQuote(name))))
		add("grant "+name, db.QueryStr(t, fmt.Sprintf(
			`SELECT grantee, table_schema, table_name, privilege_type FROM %q.information_schema.table_privileges
			 WHERE table_schema NOT IN ('pg_catalog', 'information_schema', 'crdb_internal', 'pg_extension')
			 ORDER BY 1, 2, 3, 4`, name)))
		for _, tbl := range db.QueryStr(t, fmt.Sprintf(
			`SELECT schema_name, table_name, type FROM [SHOW TABLES FROM %q] WHERE type IN ('table', 'sequence', 'view') ORDER BY 1, 2`, name)) {
			q := fmt.Sprintf(`SELECT * FROM %q.%q.%q ORDER BY 1`, name, tbl[0], tbl[1])
			if tbl[2] == "sequence" {
				q = fmt.Sprintf(`SELECT last_value FROM %q.%q.%q`, name, tbl[0], tbl[1])
			}
			add(fmt.Sprintf("rows %s.%s.%s", name, tbl[0], tbl[1]), db.QueryStr(t, q))
		}
	}
	return out
}

// withoutSkipRows leaves out the rows of app.skip, which the test checks
// on its own.
func withoutSkipRows(lines []string) []string {
	var out []string
	for _, l := range lines {
		if !strings.HasPrefix(l, "rows app.public.skip:") {
			out = append(out, l)
		}
	}
	return out
}

func lexQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// TestRestoreToPointsInTime writes data with schema changes, users, roles,
// grants, settings, drops and truncates, while the cluster takes full
// backups and makes its archive, then restores to several times into new
// clusters. Each restored cluster must equal the source at that time and
// accept new tables and writes.
func TestRestoreToPointsInTime(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)
	b := backupSection(t)
	store := openStore(t, b)
	ctx := context.Background()

	src, db := startServer(t, b)
	defer src.Stopper().Stop(ctx)
	waitFirstFull(t, store)

	type point struct {
		ts   hlc.Timestamp
		dump []string
		// skipRows is how many rows app.skip has after a restore: it is
		// marked exclude_data_from_backup from point 1 on, and restores
		// empty then, as upstream's did.
		skipRows int
	}
	var points []point
	mark := func() {
		var s string
		db.QueryRow(t, `SELECT cluster_logical_timestamp()::STRING`).Scan(&s)
		ts, err := hlc.ParseHLC(s)
		require.NoError(t, err)
		var excluded bool
		db.QueryRow(t, `SELECT create_statement LIKE '%exclude_data_from_backup = true%'
			FROM app.crdb_internal.create_statements WHERE descriptor_name = 'skip'`).Scan(&excluded)
		skipRows := 0
		if !excluded {
			db.QueryRow(t, `SELECT count(*) FROM app.skip`).Scan(&skipRows)
		}
		points = append(points, point{ts: ts, dump: withoutSkipRows(dump(t, db)), skipRows: skipRows})
	}
	// waitFull waits for a full backup that ends after the last point, so
	// that later points restore from it and earlier ones from older ones.
	waitFull := func() {
		last := points[len(points)-1].ts
		testutils.SucceedsWithin(t, func() error {
			fulls, _, err := backup.FullBackups(ctx, store)
			if err != nil {
				return err
			}
			if len(fulls) == 0 || fulls[len(fulls)-1].EndTime.LessEq(last) {
				return errors.New("no full backup after the last point yet")
			}
			return nil
		}, time.Minute)
	}

	db.Exec(t, `CREATE DATABASE app`)
	db.Exec(t, `CREATE TABLE app.t (k INT PRIMARY KEY, v STRING)`)
	db.Exec(t, `INSERT INTO app.t SELECT i, 'v' || i::STRING FROM generate_series(1, 100) AS g(i)`)
	db.Exec(t, `CREATE TABLE app.skip (k INT PRIMARY KEY)`)
	db.Exec(t, `INSERT INTO app.skip VALUES (1), (2)`)
	db.Exec(t, `CREATE TABLE app.gone (k INT PRIMARY KEY)`)
	db.Exec(t, `INSERT INTO app.gone VALUES (1)`)
	db.Exec(t, `CREATE TABLE app.trunc (k INT PRIMARY KEY)`)
	db.Exec(t, `INSERT INTO app.trunc VALUES (1), (2), (3)`)
	db.Exec(t, `CREATE USER alice`)
	db.Exec(t, `CREATE ROLE readers`)
	db.Exec(t, `GRANT SELECT ON app.t TO readers`)
	db.Exec(t, `GRANT readers TO alice`)
	db.Exec(t, `GRANT SYSTEM VIEWACTIVITY TO alice`)
	db.Exec(t, `SET CLUSTER SETTING sql.defaults.default_int_size = 4`)
	mark()
	waitFull()

	db.Exec(t, `ALTER TABLE app.t ADD COLUMN w INT NOT NULL DEFAULT 7`)
	db.Exec(t, `CREATE INDEX t_v ON app.t (v)`)
	db.Exec(t, `UPDATE app.t SET v = 'u' || k::STRING WHERE k % 3 = 0`)
	db.Exec(t, `DELETE FROM app.t WHERE k > 90`)
	db.Exec(t, `ALTER TABLE app.skip SET (exclude_data_from_backup = true)`)
	db.Exec(t, `INSERT INTO app.skip VALUES (3)`)
	db.Exec(t, `CREATE SCHEMA app.s`)
	db.Exec(t, `CREATE TYPE app.s.color AS ENUM ('red', 'green')`)
	db.Exec(t, `CREATE SEQUENCE app.seq`)
	db.Exec(t, `CREATE TABLE app.s.c (k INT PRIMARY KEY DEFAULT nextval('app.seq'), c app.s.color)`)
	db.Exec(t, `INSERT INTO app.s.c (c) VALUES ('red'), ('green'), ('red')`)
	db.Exec(t, `CREATE VIEW app.v AS SELECT k, v FROM app.t WHERE k < 10`)
	db.Exec(t, `DROP TABLE app.gone`)
	db.Exec(t, `TRUNCATE app.trunc`)
	db.Exec(t, `INSERT INTO app.trunc VALUES (10)`)
	db.Exec(t, `CREATE USER bob`)
	db.Exec(t, `GRANT INSERT ON app.s.c TO bob`)
	db.Exec(t, `REVOKE readers FROM alice`)
	db.Exec(t, `DROP DATABASE defaultdb CASCADE`)
	db.Exec(t, `CREATE DATABASE other`)
	db.Exec(t, `CREATE TABLE other.o (k INT PRIMARY KEY)`)
	db.Exec(t, `INSERT INTO other.o VALUES (5)`)
	mark()

	db.Exec(t, `ALTER TABLE app.t DROP COLUMN w`)
	db.Exec(t, `DROP INDEX app.t@t_v`)
	db.Exec(t, `INSERT INTO app.s.c (c) VALUES ('green')`)
	db.Exec(t, `REVOKE INSERT ON app.s.c FROM bob`)
	db.Exec(t, `DROP USER bob`)
	db.Exec(t, `SET CLUSTER SETTING sql.defaults.default_int_size = 8`)
	db.Exec(t, `DROP DATABASE other CASCADE`)
	mark()
	waitFull()

	db.Exec(t, `INSERT INTO app.t VALUES (1000, 'late')`)
	db.Exec(t, `UPDATE app.s.c SET c = 'green'`)
	mark()

	waitRestorable(t, store, points[len(points)-1].ts)

	restore := func(target string) (serverutils.TestServerInterface, *sqlutils.SQLRunner) {
		dst, dstDB := startServer(t, nil)
		// The statement returns once the job has finished, with its error.
		sys := sqlutils.MakeSQLRunner(dst.ApplicationLayer().SQLConn(t, serverutils.DBName("system")))
		var id int64
		sys.QueryRow(t, `SELECT crdb_internal.restore_cluster($1, $2, $3, $4, $5)`,
			b.Bucket, b.Prefix, b.Endpoint, b.Region, target).Scan(&id)
		return dst, dstDB
	}
	for i, p := range points {
		t.Run(fmt.Sprintf("point %d", i), func(t *testing.T) {
			dst, dstDB := restore(p.ts.AsOfSystemTime())
			defer dst.Stopper().Stop(ctx)
			require.Equal(t, p.dump, withoutSkipRows(dump(t, dstDB)))
			dstDB.CheckQueryResults(t, `SELECT count(*) FROM app.skip`, [][]string{{fmt.Sprint(p.skipRows)}})

			// The restored cluster takes new tables and writes.
			dstDB.Exec(t, `CREATE DATABASE IF NOT EXISTS app`)
			dstDB.Exec(t, `CREATE TABLE app.fresh (k INT PRIMARY KEY)`)
			dstDB.Exec(t, `INSERT INTO app.fresh VALUES (1)`)
			dstDB.Exec(t, `INSERT INTO app.t (k, v) VALUES (5000, 'new')`)
			dstDB.CheckQueryResults(t, `SELECT count(*) FROM app.fresh`, [][]string{{"1"}})
			dstDB.Exec(t, `CREATE USER carol`)
		})
	}

	t.Run("latest", func(t *testing.T) {
		dst, dstDB := restore("")
		defer dst.Stopper().Stop(ctx)
		require.Equal(t, points[len(points)-1].dump, withoutSkipRows(dump(t, dstDB)))
	})

	t.Run("a target after the archive", func(t *testing.T) {
		dst, _ := startServer(t, nil)
		defer dst.Stopper().Stop(ctx)
		sys := sqlutils.MakeSQLRunner(dst.ApplicationLayer().SQLConn(t, serverutils.DBName("system")))
		future := hlc.Timestamp{WallTime: timeutil.Now().Add(time.Hour).UnixNano()}
		sys.ExpectErr(t, "is after the last restorable time",
			`SELECT crdb_internal.restore_cluster($1, $2, $3, $4, $5)`,
			b.Bucket, b.Prefix, b.Endpoint, b.Region, future.AsOfSystemTime())
		// That job failed before it wrote anything, so it does not stop a
		// restore into this cluster. A second restore after that one is
		// refused.
		sys.Exec(t, `SELECT crdb_internal.restore_cluster($1, $2, $3, $4, $5)`,
			b.Bucket, b.Prefix, b.Endpoint, b.Region, "")
		sys.ExpectErr(t, "the cluster is not empty",
			`SELECT crdb_internal.restore_cluster($1, $2, $3, $4, $5)`,
			b.Bucket, b.Prefix, b.Endpoint, b.Region, "")
	})

	t.Run("a cluster that handed out an ID", func(t *testing.T) {
		dst, _ := startServer(t, nil)
		defer dst.Stopper().Stop(ctx)
		// CREATE TABLE takes its ID outside its transaction, so a rolled
		// back one leaves no table but uses up an ID that a restored table
		// would have.
		tx, err := dst.ApplicationLayer().SQLConn(t).Begin()
		require.NoError(t, err)
		_, err = tx.Exec(`CREATE TABLE defaultdb.gone (k INT PRIMARY KEY)`)
		require.NoError(t, err)
		require.NoError(t, tx.Rollback())
		sys := sqlutils.MakeSQLRunner(dst.ApplicationLayer().SQLConn(t, serverutils.DBName("system")))
		sys.ExpectErr(t, "this cluster has handed out descriptor IDs",
			`SELECT crdb_internal.restore_cluster($1, $2, $3, $4, $5)`,
			b.Bucket, b.Prefix, b.Endpoint, b.Region, "")
	})

	t.Run("a cluster that is not empty", func(t *testing.T) {
		dst, dstDB := startServer(t, nil)
		defer dst.Stopper().Stop(ctx)
		dstDB.Exec(t, `CREATE TABLE defaultdb.x (k INT PRIMARY KEY)`)
		dstDB.ExpectErr(t, "run the restore from the system database",
			`SELECT crdb_internal.restore_cluster($1, $2, $3, $4, $5)`,
			b.Bucket, b.Prefix, b.Endpoint, b.Region, "")
		sys := sqlutils.MakeSQLRunner(dst.ApplicationLayer().SQLConn(t, serverutils.DBName("system")))
		sys.ExpectErr(t, "the cluster is not empty",
			`SELECT crdb_internal.restore_cluster($1, $2, $3, $4, $5)`,
			b.Bucket, b.Prefix, b.Endpoint, b.Region, "")
	})

	t.Run("info, check and expire", func(t *testing.T) {
		var out bytes.Buffer
		require.NoError(t, backup.Info(ctx, store, &out))
		require.Contains(t, out.String(), "last restorable time: ")
		out.Reset()
		require.NoError(t, backup.Check(ctx, store, "", &out))
		require.Contains(t, out.String(), "OK")
		out.Reset()
		require.NoError(t, backup.Expire(ctx, store, 1, &out))
		require.Contains(t, out.String(), "deleted full backup")
		// The first point is before the only full backup left.
		_, err := backup.CheckTarget(ctx, store, points[0].ts)
		require.ErrorContains(t, err, "no full backup ends at or before")
		_, err = backup.CheckTarget(ctx, store, hlc.Timestamp{})
		require.NoError(t, err)
		out.Reset()
		require.NoError(t, backup.Check(ctx, store, "", &out))
	})
}
