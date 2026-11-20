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
	"fmt"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/sqlutils"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/stretchr/testify/require"
)

// TestEveryIsolationLevelRunsSerializable checks that a request for any
// isolation level, made in any of the ways SQL allows, is accepted and that
// the transaction runs SERIALIZABLE in KV.
func TestEveryIsolationLevelRunsSerializable(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	ctx := context.Background()
	srv, db, _ := serverutils.StartServer(t, base.TestServerArgs{})
	defer srv.Stopper().Stop(ctx)

	runner := sqlutils.MakeSQLRunner(db)
	runner.Exec(t, `CREATE TABLE t (k INT PRIMARY KEY)`)
	runner.Exec(t, `INSERT INTO t VALUES (1)`)

	// Each way returns the statements that open a transaction which asked for
	// the given level.
	ways := map[string]func(level string) []string{
		"BEGIN": func(level string) []string {
			return []string{"BEGIN TRANSACTION ISOLATION LEVEL " + level}
		},
		"SET TRANSACTION": func(level string) []string {
			return []string{"BEGIN", "SET TRANSACTION ISOLATION LEVEL " + level}
		},
		"transaction_isolation": func(level string) []string {
			return []string{"BEGIN", fmt.Sprintf("SET transaction_isolation = '%s'", level)}
		},
		"default_transaction_isolation": func(level string) []string {
			return []string{fmt.Sprintf("SET default_transaction_isolation = '%s'", level), "BEGIN"}
		},
		"SESSION CHARACTERISTICS": func(level string) []string {
			return []string{"SET SESSION CHARACTERISTICS AS TRANSACTION ISOLATION LEVEL " + level, "BEGIN"}
		},
	}
	levels := []string{"READ UNCOMMITTED", "READ COMMITTED", "REPEATABLE READ", "SNAPSHOT", "SERIALIZABLE"}
	for _, level := range levels {
		for name, way := range ways {
			t.Run(level+"/"+name, func(t *testing.T) {
				conn, err := db.Conn(ctx)
				require.NoError(t, err)
				defer conn.Close()
				for _, stmt := range way(level) {
					_, err := conn.ExecContext(ctx, stmt)
					require.NoError(t, err)
				}

				var shown string
				require.NoError(t, conn.QueryRowContext(ctx, "SHOW transaction_isolation").Scan(&shown))
				require.Equal(t, "serializable", shown)

				// Lock the row, then read the isolation level of the KV
				// transaction that holds the lock.
				_, err = conn.ExecContext(ctx, "SELECT * FROM t WHERE k = 1 FOR UPDATE")
				require.NoError(t, err)
				runner.CheckQueryResultsRetry(t,
					`SELECT isolation_level FROM crdb_internal.cluster_locks WHERE table_name = 't'`,
					[][]string{{"SERIALIZABLE"}},
				)

				_, err = conn.ExecContext(ctx, "ROLLBACK")
				require.NoError(t, err)
			})
		}
	}
}
