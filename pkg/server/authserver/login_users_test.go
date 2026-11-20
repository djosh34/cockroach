// Copyright 2026 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

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

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/server/apiconstants"
	"github.com/cockroachdb/cockroach/pkg/server/authserver"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/sqlutils"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
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
