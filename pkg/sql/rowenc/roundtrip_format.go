// Copyright 2020 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package rowenc

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/sql/parser"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/eval"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
)

// ParseDatumStringAs parses s as type t. This function is guaranteed to
// round-trip when printing a Datum with FmtExport. semaCtx is optional.
func ParseDatumStringAs(
	ctx context.Context, t *types.T, s string, evalCtx *eval.Context, semaCtx *tree.SemaContext,
) (tree.Datum, error) {
	switch t.Family() {
	// We use a different parser for array types because ParseAndRequireString
	// only parses the internal postgres string representation of arrays.
	case types.ArrayFamily, types.CollatedStringFamily:
		if semaCtx == nil {
			sema := tree.MakeSemaContext()
			semaCtx = &sema
		}
		return parseAsTyp(ctx, evalCtx, semaCtx, t, s)
	default:
		res, _, err := tree.ParseAndRequireString(t, s, evalCtx)
		return res, err
	}
}

func parseAsTyp(
	ctx context.Context, evalCtx *eval.Context, semaCtx *tree.SemaContext, typ *types.T, s string,
) (tree.Datum, error) {
	expr, err := parser.ParseExpr(s)
	if err != nil {
		return nil, err
	}
	typedExpr, err := tree.TypeCheck(ctx, expr, semaCtx, typ)
	if err != nil {
		return nil, err
	}
	datum, err := eval.Expr(ctx, evalCtx, typedExpr)
	return datum, err
}
