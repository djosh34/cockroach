// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package idxrecommendations

import (
	"fmt"

	"github.com/cockroachdb/cockroach/pkg/sql/opt/indexrec"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
)

// IdxRecommendations controls the generation of index recommendations
// for specific statements, and update accordingly.
type IdxRecommendations interface {
	ShouldGenerateIndexRecommendation(
		fingerprint string, planHash uint64, database string, stmtType tree.StatementType, isInternal bool,
	) bool
	UpdateIndexRecommendations(
		fingerprint string,
		planHash uint64,
		database string,
		stmtType tree.StatementType,
		isInternal bool,
		recommendations []string,
		reset bool,
	) []string
}

// FormatIdxRecommendations formats a list of index recommendations. The output
// is in the format:
//
//	{
//	  "replacement : CREATE UNIQUE INDEX ON t1 (i) STORING (k); DROP INDEX t1@existing_t1_i;",
//	  "creation : CREATE INDEX ON t2 (i) STORING (k);",
//	}
func FormatIdxRecommendations(recs []indexrec.Rec) []string {
	if len(recs) == 0 {
		return nil
	}
	recommendations := make([]string, len(recs))
	for i := range recs {
		recType := ""
		switch recs[i].RecType {
		case indexrec.TypeCreateIndex:
			recType = "creation"
		case indexrec.TypeReplaceIndex:
			recType = "replacement"
		case indexrec.TypeAlterIndex:
			recType = "alteration"
		}
		recommendations[i] = fmt.Sprintf("%s : %s", recType, recs[i].SQL)
	}

	return recommendations
}
