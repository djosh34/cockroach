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
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/testutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/sqlutils"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
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
