// Copyright 2024 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package server

import (
	"fmt"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/server/serverpb"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/sql/sqlstats"
	"github.com/cockroachdb/cockroach/pkg/testutils/datapathutils"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/datadriven"
)

func TestGetCombinedStatementsQueryClausesAndArgs(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	settings := cluster.MakeClusterSettings()
	testingKnobs := &sqlstats.TestingKnobs{}

	datadriven.RunTest(t, datapathutils.TestDataPath(t, "combined_statement_stats"), func(t *testing.T, d *datadriven.TestData) string {
		switch d.Cmd {
		case "query":
			var limit int64
			var start int64
			var end int64
			var sortString string
			if d.HasArg("sort") {
				d.ScanArgs(t, "sort", &sortString)
			}
			if d.HasArg("limit") {
				d.ScanArgs(t, "limit", &limit)
			}
			if d.HasArg("start") {
				d.ScanArgs(t, "start", &start)
			}
			if d.HasArg("end") {
				d.ScanArgs(t, "end", &end)
			}

			gotWhereClause, gotOrderAndLimitClause, gotArgs := getCombinedStatementsQueryClausesAndArgs(
				&serverpb.CombinedStatementsStatsRequest{
					Start: start,
					End:   end,
					FetchMode: &serverpb.CombinedStatementsStatsRequest_FetchMode{
						StatsType: serverpb.CombinedStatementsStatsRequest_StmtStatsOnly,
						Sort:      serverpb.StatsSortOptions(serverpb.StatsSortOptions_value[sortString]),
					},
					Limit: limit,
				},
				testingKnobs,
				false,
				settings,
			)

			return fmt.Sprintf("--WHERE--\n%s\n--ORDER AND LIMIT--\n%s\n--ARGS--\n%v", gotWhereClause, gotOrderAndLimitClause, gotArgs)
		default:
			t.Fatalf("unknown cmd: %s", d.Cmd)
		}
		return ""
	})
}
