// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package upgrades_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/testutils/skip"
	"github.com/cockroachdb/cockroach/pkg/testutils/testcluster"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSQLStatsTTLChange is testing the permanent upgrade associated with
// Permanent_V23_1ChangeSQLStatsTTL. We no longer support versions this old, but
// we still need to test that the upgrade happens as expected when creating a
// new cluster.
func TestSQLStatsTTLChange(t *testing.T) {
	skip.UnderStressRace(t)
	defer leaktest.AfterTest(t)()

	ctx := context.Background()
	tc := testcluster.StartTestCluster(t, 1, base.TestClusterArgs{})
	defer tc.Stopper().Stop(ctx)
	db := tc.ServerConn(0)
	defer db.Close()

	tables := []string{
		"system.public.statement_statistics",
		"system.public.transaction_statistics",
		"system.public.statement_activity",
		"system.public.transaction_activity",
	}

	var target string
	var rawConfigSql string
	for _, table := range tables {
		row := db.QueryRow(fmt.Sprintf("SHOW ZONE CONFIGURATION FROM TABLE %s", table))
		err := row.Scan(&target, &rawConfigSql)
		require.NoError(t, err)

		assert.Equal(t, target, fmt.Sprintf("TABLE %s", table))
		assert.Contains(t, rawConfigSql, "gc.ttlseconds = 3600")
	}
}
