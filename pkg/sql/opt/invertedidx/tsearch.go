// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package invertedidx

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/sql/inverted"
	"github.com/cockroachdb/cockroach/pkg/sql/opt"
	"github.com/cockroachdb/cockroach/pkg/sql/opt/cat"
	"github.com/cockroachdb/cockroach/pkg/sql/opt/memo"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/eval"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
	"github.com/cockroachdb/errors"
)

type tsqueryFilterPlanner struct {
	tabID           opt.TableID
	index           cat.Index
	computedColumns map[opt.ColumnID]opt.ScalarExpr
}

var _ invertedFilterPlanner = &tsqueryFilterPlanner{}

// extractInvertedFilterConditionFromLeaf implements the invertedFilterPlanner
// interface.
func (t *tsqueryFilterPlanner) extractInvertedFilterConditionFromLeaf(
	_ context.Context, _ *eval.Context, expr opt.ScalarExpr,
) (invertedExpr inverted.Expression, remainingFilters opt.ScalarExpr) {
	var constantVal opt.ScalarExpr
	var left, right opt.ScalarExpr
	switch e := expr.(type) {
	case *memo.TSMatchesExpr:
		left, right = e.Left, e.Right
	default:
		// Only the above types are supported.
		return inverted.NonInvertedColExpression{}, expr
	}
	if isIndexColumn(t.tabID, t.index, left, t.computedColumns) && memo.CanExtractConstDatum(right) {
		constantVal = right
	} else if isIndexColumn(t.tabID, t.index, right, t.computedColumns) && memo.CanExtractConstDatum(left) {
		constantVal = left
	} else {
		// Can only accelerate with a single constant value.
		return inverted.NonInvertedColExpression{}, expr
	}
	d := memo.ExtractConstDatum(constantVal)
	if d.ResolvedType() != types.TSQuery {
		panic(errors.AssertionFailedf(
			"trying to apply tsvector inverted index to unsupported type %s", d.ResolvedType().SQLStringForError(),
		))
	}
	q := d.(*tree.DTSQuery).TSQuery
	var err error
	invertedExpr, err = q.GetInvertedExpr()
	if err != nil {
		// An inverted expression could not be extracted.
		return inverted.NonInvertedColExpression{}, expr
	}

	// If the extracted inverted expression is not tight then remaining filters
	// must be applied after the inverted index scan.
	// TODO(jordan): we could do better here by pruning terms that we successfully
	// turn into inverted expressions during the tsquery tree walk. We'd need to
	// implement a function that removes a term from a tsquery tree.
	if !invertedExpr.IsTight() {
		remainingFilters = expr
	}

	return invertedExpr, remainingFilters
}
