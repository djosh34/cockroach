// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package lookupjoin

import (
	"github.com/cockroachdb/cockroach/pkg/sql/opt"
	"github.com/cockroachdb/cockroach/pkg/sql/opt/memo"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/eval"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
)

// HasJoinFilterConstants returns true if the filter constrains the given column
// to a constant, non-NULL value or set of constant, non-NULL values.
func HasJoinFilterConstants(
	filters memo.FiltersExpr, col opt.ColumnID, evalCtx *eval.Context,
) bool {
	for filterIdx := range filters {
		props := filters[filterIdx].ScalarProps()
		if props.TightConstraints {
			if ok := props.Constraints.HasSingleColumnNonNullConstValues(evalCtx, col); ok {
				return true
			}
		}
	}
	return false
}

// FindJoinFilterConstants tries to find a filter that is exactly equivalent to
// constraining the given column to a constant value or a set of constant
// values. If successful, the constant values and the index of the constraining
// FiltersItem are returned. If multiple filters match, the one that minimizes
// the number of returned values is chosen. Note that the returned constant
// values do not contain NULL.
func FindJoinFilterConstants(
	filters memo.FiltersExpr, col opt.ColumnID, evalCtx *eval.Context,
) (values tree.Datums, filterIdx int, ok bool) {
	var bestValues tree.Datums
	var bestFilterIdx int
	for filterIdx := range filters {
		props := filters[filterIdx].ScalarProps()
		if props.TightConstraints {
			constVals, ok := props.Constraints.ExtractSingleColumnNonNullConstValues(evalCtx, col)
			if !ok {
				continue
			}
			if bestValues == nil || len(bestValues) > len(constVals) {
				bestValues = constVals
				bestFilterIdx = filterIdx
			}
		}
	}
	if bestValues == nil {
		return nil, -1, false
	}
	return bestValues, bestFilterIdx, true
}
