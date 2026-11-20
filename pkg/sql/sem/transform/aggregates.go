// Copyright 2017 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package transform

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/sessiondata"
)

// isAggregateVisitor checks if walked expressions contain aggregate functions.
type isAggregateVisitor struct {
	Aggregated bool
	// searchPath is used to search for unqualified function names.
	searchPath sessiondata.SearchPath
	ctx        context.Context
}

var _ tree.Visitor = &isAggregateVisitor{}

// VisitPre satisfies the Visitor interface.
func (v *isAggregateVisitor) VisitPre(expr tree.Expr) (recurse bool, newExpr tree.Expr) {
	switch t := expr.(type) {
	case *tree.FuncExpr:
		if t.IsWindowFunctionApplication() {
			// A window function application of an aggregate builtin is not an
			// aggregate function, but it can contain aggregate functions.
			return true, expr
		}

		// We don't currently support aggregate UDFs, so no resolver is required
		// here.
		fd, err := t.Func.Resolve(v.ctx, &v.searchPath, nil /* resolver */)
		if err != nil {
			return false, expr
		}
		funcCls, err := fd.GetClass()
		if err != nil {
			return false, expr
		}

		if funcCls == tree.AggregateClass {
			v.Aggregated = true
			return false, expr
		}
	case *tree.Subquery:
		return false, expr
	}

	return true, expr
}

// VisitPost satisfies the Visitor interface.
func (*isAggregateVisitor) VisitPost(expr tree.Expr) tree.Expr { return expr }
