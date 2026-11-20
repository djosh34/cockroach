// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package fingerprintutils_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/testutils/fingerprintutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/sqlutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/testcluster"
	"github.com/cockroachdb/cockroach/pkg/util/hlc"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
	"github.com/stretchr/testify/require"
)

// TestFingerprintUtility asserts that the fingerprint helpers in the package can indeed catch
// fingerprint mismatches and return a proper error message.
func TestFingerprintUtility(t *testing.T) {
	defer leaktest.AfterTest(t)()

	ctx := context.Background()
	tc := testcluster.StartTestCluster(t, 1, base.TestClusterArgs{})
	defer tc.Stopper().Stop(ctx)
	db := tc.ServerConn(0)
	sql := sqlutils.MakeSQLRunner(db)

	startTime := hlc.Timestamp{WallTime: timeutil.Now().UnixNano()}
	setupDatabase := func(name string) {
		sql.Exec(t, fmt.Sprintf(`CREATE DATABASE %s`, name))
		tableName := name + ".t1"
		sql.Exec(t, fmt.Sprintf(`CREATE TABLE %s (id INT PRIMARY KEY, s STRING)`, tableName))
		sql.Exec(t, fmt.Sprintf(`INSERT INTO %s VALUES (1, 'x'),(2,'y')`, tableName))
	}
	setupDatabase("d1")
	setupDatabase("d2")

	// Fingerprints should only match on stripped fingerprint.
	fingerprintDatabases := func(optFuncs ...func(*fingerprintutils.FingerprintOption)) error {
		f1Stripped, err := fingerprintutils.FingerprintDatabase(ctx, db, "d1", optFuncs...)
		require.NoError(t, err)

		f2Stripped, err := fingerprintutils.FingerprintDatabase(ctx, db, "d2", optFuncs...)
		require.NoError(t, err)
		return fingerprintutils.CompareDatabaseFingerprints(f1Stripped, f2Stripped)
	}

	require.NoError(t, fingerprintDatabases(fingerprintutils.Stripped()))
	require.ErrorContains(t, fingerprintDatabases(fingerprintutils.StartTime(startTime)),
		`fingerprint mismatch on "t1" table`)

	// Ensure cluster stripped fingerprint mismatch occurs after adding a single row
	// Don't include system db, as it may naturally change over time.
	clusterFingerprint, err := fingerprintutils.FingerprintAllDatabases(ctx, db, false, fingerprintutils.Stripped())
	require.NoError(t, err)
	sql.Exec(t, `INSERT INTO d1.t1 VALUES (3,'z')`)

	clusterFingerprintAfterInsert, err := fingerprintutils.FingerprintAllDatabases(ctx, db, false, fingerprintutils.Stripped())
	require.NoError(t, err)
	require.ErrorContains(t, fingerprintutils.CompareMultipleDatabaseFingerprints(clusterFingerprint,
		clusterFingerprintAfterInsert), `fingerprint mismatch on "t1" table`)

	// Ensure one cannot run nonsensical fingerprint cmds.
	_, err = fingerprintutils.FingerprintDatabase(ctx, db, "d1", fingerprintutils.Stripped(), fingerprintutils.StartTime(startTime))
	require.ErrorContains(t, err, "cannot specify stripped and a start time")

	_, err = fingerprintutils.FingerprintDatabase(ctx, db, "d1", fingerprintutils.Stripped(), fingerprintutils.RevisionHistory())
	require.ErrorContains(t, err, "cannot specify stripped and revision history")
}
