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
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/testutils/skip"
	"github.com/cockroachdb/cockroach/pkg/testutils/testcluster"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/stretchr/testify/assert"
)

// TestCreateActivityUpdateJobMigration is testing the permanent upgrade
// associated Permanent_V23_1CreateSystemActivityUpdateJob. We no longer support
// versions this old, but we still need to test that the upgrade happens as
// expected when creating a new cluster.
func TestCreateActivityUpdateJobMigration(t *testing.T) {
	skip.UnderStressRace(t)
	defer leaktest.AfterTest(t)()
	ctx := context.Background()

	tc := testcluster.StartTestCluster(t, 1, base.TestClusterArgs{})
	defer tc.Stopper().Stop(ctx)

	db := tc.ServerConn(0)
	defer db.Close()

	row := db.QueryRow("SELECT count(*) FROM system.public.jobs WHERE id = 103")
	assert.NotNil(t, row)
	assert.NoError(t, row.Err())
	var count int
	err := row.Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, 1, count)
}
