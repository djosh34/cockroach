// Copyright 2026 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

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
	defer func() { require.NoError(t, conn.Close()) }()
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
