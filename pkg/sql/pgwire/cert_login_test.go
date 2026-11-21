// Copyright 2026 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package pgwire_test

import (
	"context"
	"net/url"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/sqlutils"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

// TestLoginNeedsClientCert checks that a secure server admits SQL clients
// only with a client cert whose client rule names the requested user. The
// default test rules map each test cert's CN to the role of that name.
func TestLoginNeedsClientCert(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)
	ctx := context.Background()

	// The test dials the server's SQL address, which serves the system
	// tenant.
	srv, db, _ := serverutils.StartServer(t, base.TestServerArgs{
		DefaultTestTenant: base.TestIsSpecificToStorageLayerAndNeedsASystemTenant,
	})
	defer srv.Stopper().Stop(ctx)
	s := srv.ApplicationLayer()
	_, err := db.Exec("CREATE USER testuser WITH PASSWORD 'secret'")
	require.NoError(t, err)

	connect := func(user *url.Userinfo, withCert bool, certUser string) error {
		u, cleanup := sqlutils.PGUrlWithOptionalClientCerts(t, s.AdvSQLAddr(), t.Name(), user, withCert, certUser)
		defer cleanup()
		conn, err := pgx.Connect(ctx, u.String())
		if err != nil {
			return err
		}
		return conn.Close(ctx)
	}

	// The cert of testuser logs in as testuser.
	require.NoError(t, connect(url.User("testuser"), true, ""))

	// A password does not replace the client cert.
	err = connect(url.UserPassword("testuser", "secret"), false, "")
	require.ErrorContains(t, err, "certificate required")

	// The cert of testuser does not log in as root.
	err = connect(url.User("root"), true, "testuser")
	require.ErrorContains(t, err, "no client rule maps certificate")

	// A rule for a user that does not exist does not log in.
	err = connect(url.User("bar"), true, "")
	require.ErrorContains(t, err, "user bar does not exist")
}
