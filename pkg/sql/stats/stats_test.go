// Copyright 2024 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package stats

import (
	"context"
	"fmt"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/colinfo"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/sqlutils"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/randutil"
	"github.com/lib/pq/oid"
)

// TestStatsAnyType verifies that table stats collection on a column of any type
// works.
func TestStatsAnyType(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	rng, _ := randutil.NewTestRand()
	ctx := context.Background()
	srv, sqlDB, _ := serverutils.StartServer(t, base.TestServerArgs{})
	defer srv.Stopper().Stop(ctx)
	st := srv.ApplicationLayer().ClusterSettings()
	runner := sqlutils.MakeSQLRunner(sqlDB)
	formatter := tree.NewFmtCtx(tree.FmtParsable)

	for i := 0; i < 10; i++ {
		var typ *types.T
	loop:
		for {
			typ = RandType(rng)
			switch typ.Oid() {
			case oid.T_int2vector, oid.T_oidvector:
				// We can't create a table with a column of this type.
				continue loop
			case oid.T_regtype:
				// Casting random integers to REGTYPE might fail.
				continue loop
			}
			if err := colinfo.ValidateColumnDefType(ctx, st.Version, typ); err == nil {
				break
			}
		}
		numRows := 1 + rng.Intn(10)
		t.Logf("picked %s and %d rows", typ.SQLString(), numRows)
		tableName := fmt.Sprintf("t%d", i)
		runner.Exec(t, fmt.Sprintf("CREATE TABLE %s(c %s)", tableName, typ.SQLString()))
		for j := 0; j < numRows; j++ {
			d := RandDatum(rng, typ, false /* nullOk */)
			formatter.Reset()
			formatter.FormatNode(d)
			runner.Exec(t, fmt.Sprintf("INSERT INTO %s VALUES (%s::%s)", tableName, formatter.String(), typ.SQLString()))
		}
		runner.Exec(t, "ANALYZE "+tableName)
	}
}
