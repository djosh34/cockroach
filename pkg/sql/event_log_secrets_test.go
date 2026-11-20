// Copyright 2026 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package sql_test

import (
	"context"
	"strings"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/testutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/sqlutils"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEventLogKeepsSecrets checks that the event log records neither the value
// of a sensitive setting nor a password bound to a placeholder.
func TestEventLogKeepsSecrets(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	srv, db, _ := serverutils.StartServer(t, base.TestServerArgs{})
	defer srv.Stopper().Stop(context.Background())
	r := sqlutils.MakeSQLRunner(db)

	r.Exec(t, `SET CLUSTER SETTING cloudstorage.http.custom_ca = 'SECRETCA'`)
	r.Exec(t, `CREATE ROLE event_role WITH LOGIN PASSWORD $1`, "SECRETPW")
	r.Exec(t, `ALTER ROLE event_role WITH PASSWORD $1`, "SECRETPW2")

	testutils.SucceedsSoon(t, func() error {
		var n int
		r.QueryRow(t, `SELECT count(*) FROM system.eventlog
			WHERE "eventType" IN ('set_cluster_setting', 'create_role', 'alter_role')
			AND (info LIKE '%event_role%' OR info LIKE '%custom_ca%')`).Scan(&n)
		if n < 3 {
			return errors.Newf("found %d of 3 events", n)
		}
		return nil
	})
	rows := r.QueryStr(t, `SELECT info FROM system.eventlog`)
	for _, row := range rows {
		for _, secret := range []string{"SECRETCA", "SECRETPW"} {
			assert.NotContains(t, row[0], secret)
		}
	}
}

// TestStatementStatisticsKeepSecrets checks that SQL statistics hold neither a
// password nor the credentials in a URI.
func TestStatementStatisticsKeepSecrets(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	srv, db, _ := serverutils.StartServer(t, base.TestServerArgs{ExternalIODir: t.TempDir()})
	defer srv.Stopper().Stop(context.Background())
	r := sqlutils.MakeSQLRunner(db)

	r.Exec(t, `CREATE ROLE stats_role WITH LOGIN PASSWORD 'SECRETPW'`)
	r.Exec(t, `CREATE TABLE stats_t (a INT)`)
	_, err := db.Exec(`IMPORT INTO stats_t CSV DATA ('nodelocal://1/missing.csv?AWS_SECRET_ACCESS_KEY=SECRETKEY')`)
	require.Error(t, err)
	r.Exec(t, `CREATE EXTERNAL CONNECTION stats_conn AS 'nodelocal://1/SECRETPATH'`)

	for _, query := range []string{
		`SELECT metadata::STRING, statistics::STRING FROM crdb_internal.statement_statistics`,
		`SELECT key, last_error FROM crdb_internal.node_statement_statistics`,
	} {
		for _, row := range r.QueryStr(t, query) {
			for _, secret := range []string{"SECRETPW", "SECRETKEY", "SECRETPATH"} {
				assert.NotContains(t, strings.Join(row, " "), secret, query)
			}
		}
	}
}
