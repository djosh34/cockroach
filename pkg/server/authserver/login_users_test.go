// Copyright 2026 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package authserver_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/cockroach/pkg/server/apiconstants"
	"github.com/cockroachdb/cockroach/pkg/server/authserver"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/sqlutils"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
	"github.com/stretchr/testify/require"
)

// TestLoginDoesNotRevealUsers checks that both HTTP password logins fail the
// same way for a wrong password, a user who doesn't exist, a user who may not
// log in and a user whose password expired.
func TestLoginDoesNotRevealUsers(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)
	ctx := context.Background()

	s := serverutils.StartServerOnly(t, base.TestServerArgs{})
	defer s.Stopper().Stop(ctx)
	// The API v2 login is only served by the system tenant.
	ts := s.SystemLayer()
	sqlDB := sqlutils.MakeSQLRunner(ts.SQLConn(t))
	sqlDB.Exec(t, `CREATE USER known WITH PASSWORD 'pw1'`)
	sqlDB.Exec(t, `CREATE USER nologin WITH NOLOGIN PASSWORD 'pw1'`)
	sqlDB.Exec(t, `CREATE USER expired WITH PASSWORD 'pw1' VALID UNTIL '2000-01-01'`)

	httpClient, err := ts.GetUnauthenticatedHTTPClient()
	require.NoError(t, err)
	reply := func(resp *http.Response, err error) string {
		require.NoError(t, err)
		defer func() { require.NoError(t, resp.Body.Close()) }()
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		return fmt.Sprintf("%d %s", resp.StatusCode, body)
	}
	login := func(user, password string) string {
		req, err := json.Marshal(map[string]string{"username": user, "password": password})
		require.NoError(t, err)
		v1 := reply(httpClient.Post(ts.AdminURL().WithPath(authserver.LoginPath).String(),
			"application/json", strings.NewReader(string(req))))
		v2 := reply(httpClient.PostForm(ts.AdminURL().WithPath(apiconstants.APIV2Path+"login/").String(),
			url.Values{"username": {user}, "password": {password}}))
		return v1 + "\n" + v2
	}

	want := login("known", "wrong")
	require.Equal(t, 2, strings.Count(want, "401 "), want)
	for _, user := range []string{"nobody", "nologin", "expired"} {
		require.Equal(t, want, login(user, "pw1"), "user %s", user)
	}
}

// TestLoginTakesAsLongForUnknownUsers checks that a login for a user who
// doesn't exist does the same hashing work as a wrong password for a real
// user, so that its timing doesn't tell which users exist.
func TestLoginTakesAsLongForUnknownUsers(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)
	ctx := context.Background()

	s, db, _ := serverutils.StartServer(t, base.TestServerArgs{})
	defer s.Stopper().Stop(ctx)
	sqlDB := sqlutils.MakeSQLRunner(db)
	// A costly hash makes the hashing work stand out from the user lookup.
	sqlDB.Exec(t, `SET CLUSTER SETTING server.user_login.password_hashes.default_cost.scram_sha_256 = 500000`)
	sqlDB.Exec(t, `CREATE USER known WITH PASSWORD 'pw1'`)

	authServer := s.ApplicationLayer().HTTPAuthServer().(authserver.Server)
	// login returns the shortest of three login times, which leaves out pauses
	// that have nothing to do with the login.
	login := func(user string) time.Duration {
		var shortest time.Duration
		for i := 0; i < 3; i++ {
			start := timeutil.Now()
			valid, err := authServer.VerifyPasswordDBConsole(ctx, username.MakeSQLUsernameFromPreNormalizedString(user), "wrong")
			require.NoError(t, err)
			require.False(t, valid)
			if d := timeutil.Since(start); i == 0 || d < shortest {
				shortest = d
			}
		}
		return shortest
	}
	// The first logins also make the dummy hash and fill the caches.
	login("nobody")
	known, unknown := login("known"), login("nobody")
	require.Greater(t, unknown, known/2, "known user %s, unknown user %s", known, unknown)
}
