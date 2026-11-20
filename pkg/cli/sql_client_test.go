// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package cli

import (
	"context"
	"net/url"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/cli/clienturl"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/security/clientsecopts"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/catconstants"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestMakeTenantSQLClientRetry(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	// Separate process multi-tenancy does not support the
	// ccluster option.
	ctx := context.Background()
	vcName := "test-name"
	c := NewCLITest(TestCLIParams{
		TenantArgs: &base.TestTenantArgs{
			TenantName: roachpb.TenantName(vcName),
			TenantID:   serverutils.TestTenantID(),
		},
		Insecure: true,
	})
	defer c.Cleanup()

	// Create a cliContext for the testserver's PG URL.
	testCliCtx := &cliContext{
		Config: &base.Config{
			Insecure: true,
		},
		clientOpts: clientsecopts.ClientOptions{
			ClientSecurityOptions: clientsecopts.ClientSecurityOptions{
				Insecure: true,
			},
		},
	}
	u := clienturl.NewURLParser(&cobra.Command{}, &testCliCtx.clientOpts, false, func(string, ...interface{}) {})
	url := url.URL{Scheme: "postgres", Host: c.getSQLAddr()}
	require.NoError(t, u.Set(url.String()))

	t.Run("failures connecting to system tenant are retried against the default tenant", func(t *testing.T) {
		conn, err := testCliCtx.makeTenantSQLClient(ctx, "test", useDefaultDb, catconstants.SystemTenantName)
		require.NoError(t, err)
		defer func() { _ = conn.Close() }()
		require.NoError(t, conn.EnsureConn(ctx))
		row, err := conn.QueryRow(ctx, "SHOW virtual_cluster_name")
		require.NoError(t, err)
		require.Equal(t, vcName, row[0].(string))
	})
	t.Run("failures connecting to non-system tenant are not retried", func(t *testing.T) {
		conn, err := testCliCtx.makeTenantSQLClient(ctx, "test", useDefaultDb, "doesnotexists")
		require.NoError(t, err)
		defer func() { _ = conn.Close() }()

		err = conn.EnsureConn(ctx)
		require.Error(t, err)
		require.True(t, maybeTenantSelectionNotSupportedErr(err))
	})
}
