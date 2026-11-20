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
	"github.com/cockroachdb/cockroach/pkg/sql/sessiondatapb"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/sqlutils"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/protoutil"
	"github.com/stretchr/testify/require"
)

// TestDeserializeSessionCannotSetSuperuser checks that a user who is not an
// admin cannot turn on the read-only is_superuser variable with a crafted
// session.
func TestDeserializeSessionCannotSetSuperuser(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	srv, db, _ := serverutils.StartServer(t, base.TestServerArgs{})
	defer srv.Stopper().Stop(context.Background())
	sqlutils.MakeSQLRunner(db).Exec(t, `CREATE USER testuser`)

	ctx := context.Background()
	// One connection, so that the session that deserializes is the one asked.
	conn, err := srv.ApplicationLayer().SQLConn(t, serverutils.User("testuser")).Conn(ctx)
	require.NoError(t, err)
	defer conn.Close()
	user := sqlutils.MakeSQLRunner(conn)

	var state []byte
	user.QueryRow(t, `SELECT crdb_internal.serialize_session()`).Scan(&state)
	var m sessiondatapb.MigratableSession
	require.NoError(t, protoutil.Unmarshal(state, &m))
	require.False(t, m.LocalOnlySessionData.IsSuperuser)
	m.LocalOnlySessionData.IsSuperuser = true
	crafted, err := protoutil.Marshal(&m)
	require.NoError(t, err)

	user.Exec(t, `SELECT crdb_internal.deserialize_session($1)`, crafted)
	user.CheckQueryResults(t, `SHOW is_superuser`, [][]string{{"off"}})
}
