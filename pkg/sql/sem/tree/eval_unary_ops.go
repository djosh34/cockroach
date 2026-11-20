// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package tree

import "context"

// UnaryNoop is a UnaryEvalOp.
type UnaryNoop struct{}

// Eval of UnaryNoop does nothing and returns the passed Datum.
func (v *UnaryNoop) Eval(ctx context.Context, evaluator OpEvaluator, d Datum) (Datum, error) {
	return d, nil
}

type (
	// UnaryMinusFloatOp is a UnaryEvalOp.
	UnaryMinusFloatOp struct{}
	// UnaryMinusIntervalOp is a UnaryEvalOp.
	UnaryMinusIntervalOp struct{}
	// UnaryMinusIntOp is a UnaryEvalOp.
	UnaryMinusIntOp struct{}
	// UnaryMinusDecimalOp is a UnaryEvalOp.
	UnaryMinusDecimalOp struct{}
)
type (
	// ComplementIntOp is a UnaryEvalOp.
	ComplementIntOp struct{}
	// ComplementVarBitOp is a UnaryEvalOp.
	ComplementVarBitOp struct{}
	// ComplementINetOp is a UnaryEvalOp.
	ComplementINetOp struct{}
)
type (
	// SqrtFloatOp is a UnaryEvalOp.
	SqrtFloatOp struct{}
	// SqrtDecimalOp is a UnaryEvalOp.
	SqrtDecimalOp struct{}
	// CbrtFloatOp is a UnaryEvalOp.
	CbrtFloatOp struct{}
	// CbrtDecimalOp is a UnaryEvalOp.
	CbrtDecimalOp struct{}
)
