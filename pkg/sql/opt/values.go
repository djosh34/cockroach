// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package opt

import (
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
)

// LiteralRows is a container for literal values (i.e. Datums).
type LiteralRows struct {
	Rows tree.ExprContainer
}

var _ ScalarExpr = &LiteralRows{}

// Op returns the operator type of the expression.
func (t LiteralRows) Op() Operator {
	return AnyScalarOp
}

// ChildCount returns the number of children of the expression.
func (t LiteralRows) ChildCount() int {
	return 0
}

// Child returns the nth child of the expression.
func (t LiteralRows) Child(nth int) Expr {
	return nil
}

// Private returns operator-specific data. Callers are expected to know the
// type and format of the data, which will differ from operator to operator.
// For example, an operator may choose to return one of its fields, or perhaps
// a pointer to itself, or nil if there is nothing useful to return.
func (t LiteralRows) Private() interface{} {
	return nil
}

// String returns a human-readable string representation for the expression
// that can be used for debugging and testing.
func (t LiteralRows) String() string {
	return ""
}

// Rank is part of ScalarExpr interface
func (t LiteralRows) Rank() ScalarRank {
	return 0
}

// DataType is part of ScalarExpr interface
func (t LiteralRows) DataType() *types.T {
	return nil
}
