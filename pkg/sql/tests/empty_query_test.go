// Copyright 2021 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package tests

import (
	"context"
	"net/url"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/sqlutils"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

// TestEmptyQuery is a regression test to ensure that sending an empty
// query to the database as the first query is safe.
func TestEmptyQuery(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	s := serverutils.StartServerOnly(t, base.TestServerArgs{Insecure: false, UseDatabase: "defaultdb"})
	defer s.Stopper().Stop(context.Background())

	pgURL, cleanupFunc := sqlutils.PGUrl(
		t, s.ApplicationLayer().AdvSQLAddr(), "testConnClose" /* prefix */, url.User(username.RootUser),
	)
	defer cleanupFunc()

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, pgURL.String())
	require.NoError(t, err)

	err = conn.QueryRow(ctx, "").Scan()
	require.Error(t, err)
	require.Regexp(t, "no rows in result set", err)
}
