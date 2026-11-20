// Copyright 2018 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package tree

import (
	"github.com/cockroachdb/cockroach/pkg/sql/sem/cast"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/volatility"
)

// OperatorIsImmutable returns true if the given expression corresponds to a
// constant operator. Note importantly that this will return true for all
// expr types other than FuncExpr, CastExpr, UnaryExpr, BinaryExpr, and
// ComparisonExpr. It does not do any recursive searching.
func OperatorIsImmutable(expr Expr) bool {
	switch t := expr.(type) {
	case *FuncExpr:
		return t.ResolvedOverload().Class == NormalClass && t.fn.Volatility <= volatility.Immutable

	case *CastExpr:
		v, ok := cast.LookupCastVolatility(t.Expr.(TypedExpr).ResolvedType(), t.typ)
		return ok && v <= volatility.Immutable

	case *UnaryExpr:
		return t.op.Volatility <= volatility.Immutable

	case *BinaryExpr:
		return t.Op.Volatility <= volatility.Immutable

	case *ComparisonExpr:
		return t.Op.Volatility <= volatility.Immutable

	default:
		return true
	}
}
