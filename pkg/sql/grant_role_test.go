// Copyright 2022 The Cockroach Authors.
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

	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/desctestutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/sqlutils"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/stretchr/testify/require"
)

// TestNoOpGrantRole tests that if we GRANT membership of a role to a user who
// is already a member of that role will not cause any schema change to the
// `system.role_members` table.
func TestNoOpGrantRole(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)
	params, _ := createTestServerParams()
	s, sqlDB, kvDB := serverutils.StartServer(t, params)
	defer s.Stopper().Stop(context.Background())
	tdb := sqlutils.MakeSQLRunner(sqlDB)

	tdb.Exec(t, "CREATE USER developer WITH CREATEDB")
	tdb.Exec(t, "CREATE USER roach WITH PASSWORD NULL")

	// Grant membership of role `developer` to user `roach`.
	// Assert that user `roach` is now a member of role `developer` by peeking the `system.role_members` table.
	tdb.Exec(t, "GRANT developer TO roach")
	num := 0
	tdb.QueryRow(t, "SELECT count(1) FROM system.role_members WHERE role = 'developer' AND member = 'roach'").Scan(&num)
	require.Equal(t, 1, num)
	roleMembersTableDesc := desctestutils.TestingGetTableDescriptor(kvDB, keys.SystemSQLCodec, "system", "public", "role_members")
	roleMembersTableVersion := roleMembersTableDesc.GetVersion()

	// Repeat the statement and assert that no schema change was performed to table `system.role_members`
	// by checking the table's version remains the same.
	tdb.Exec(t, "GRANT developer TO roach")
	tdb.QueryRow(t, "SELECT count(1) FROM system.role_members WHERE role = 'developer' AND member = 'roach'").Scan(&num)
	require.Equal(t, 1, num)
	roleMembersTableDesc = desctestutils.TestingGetTableDescriptor(kvDB, keys.SystemSQLCodec, "system", "public", "role_members")
	require.Equal(t, roleMembersTableVersion, roleMembersTableDesc.GetVersion())
}
