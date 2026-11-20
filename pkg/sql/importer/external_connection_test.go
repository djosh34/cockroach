// Copyright 2026 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package importer_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/cloud"
	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/cockroach/pkg/sql/isql"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/sqlutils"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/stretchr/testify/require"
)

// TestExternalConnectionChecks checks that a path under an external://
// connection cannot reach above the connection's base path, and that using a
// connection always needs the USAGE privilege, also for EXPORT.
func TestExternalConnectionChecks(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)
	ctx := context.Background()

	dir := t.TempDir()
	for _, sub := range []string{"base", "secret"} {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, sub), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, sub, "data.csv"), []byte("1\n"), 0644))
	}

	s, db, _ := serverutils.StartServer(t, base.TestServerArgs{ExternalIODir: dir})
	defer s.Stopper().Stop(ctx)
	sqlDB := sqlutils.MakeSQLRunner(db)
	sqlDB.Exec(t, `CREATE EXTERNAL CONNECTION conn AS 'nodelocal://1/base'`)
	sqlDB.Exec(t, `CREATE TABLE t (a INT8)`)

	t.Run("path stays under the base path", func(t *testing.T) {
		sqlDB.Exec(t, `IMPORT INTO t CSV DATA ('external://conn/data.csv')`)
		sqlDB.ExpectErr(t, `must not contain ".."`,
			`IMPORT INTO t CSV DATA ('external://conn/../secret/data.csv')`)
	})

	t.Run("KMS needs USAGE", func(t *testing.T) {
		sqlDB.Exec(t, `CREATE USER kmsuser`)
		env := &cloud.TestKMSEnv{
			Settings:         s.ClusterSettings(),
			ExternalIOConfig: &base.ExternalIODirConfig{},
			DB:               s.InternalDB().(isql.DB),
			Username:         username.MakeSQLUsernameFromPreNormalizedString("kmsuser"),
		}
		// No KMS is contacted: the check comes before the connection is loaded.
		_, err := cloud.KMSFromURI(ctx, "external://conn", env)
		require.ErrorContains(t, err, "does not have USAGE privilege on external connection conn")
	})

	t.Run("EXPORT needs USAGE", func(t *testing.T) {
		sqlDB.Exec(t, `CREATE USER testuser`)
		sqlDB.Exec(t, `GRANT SELECT ON t TO testuser`)
		testuser := sqlutils.MakeSQLRunner(s.ApplicationLayer().SQLConn(t, serverutils.User("testuser")))
		testuser.ExpectErr(t, "does not have USAGE privilege on external connection conn",
			`EXPORT INTO CSV 'external://conn/out' FROM TABLE t`)
		// USAGE through a role counts.
		sqlDB.Exec(t, `CREATE ROLE conn_users`)
		sqlDB.Exec(t, `GRANT USAGE ON EXTERNAL CONNECTION conn TO conn_users`)
		sqlDB.Exec(t, `GRANT conn_users TO testuser`)
		testuser.Exec(t, `EXPORT INTO CSV 'external://conn/out' FROM TABLE t`)
	})
}
