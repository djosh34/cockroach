// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package colexecprojconst

import (
	"reflect"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/sql/colexecop"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree/treebin"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree/treecmp"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
)

func TestGetProjectionConstOperator(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)
	binOp := treebin.MakeBinaryOperator(treebin.Mult)
	var input colexecop.Operator
	colIdx := 3
	inputTypes := make([]*types.T, colIdx+1)
	inputTypes[colIdx] = types.Float
	constVal := 31.37
	constArg := tree.NewDFloat(tree.DFloat(constVal))
	outputIdx := 5
	calledOnNullInput := false
	op, err := GetProjectionRConstOperator(
		nil /* allocator */, inputTypes, types.Float, types.Float, binOp, input, colIdx,
		constArg, outputIdx, nil /* EvalCtx */, nil /* BinFn */, nil /* cmpExpr */, calledOnNullInput,
	)
	if err != nil {
		t.Error(err)
	}
	expected := &projMultFloat64Float64ConstOp{
		projConstOpBase: projConstOpBase{
			OneInputHelper:    colexecop.MakeOneInputHelper(op.(*projMultFloat64Float64ConstOp).Input),
			colIdx:            colIdx,
			outputIdx:         outputIdx,
			calledOnNullInput: calledOnNullInput,
		},
		constArg: constVal,
	}
	if !reflect.DeepEqual(op, expected) {
		t.Errorf("got %+v,\nexpected %+v", op, expected)
	}
}

func TestGetProjectionConstMixedTypeOperator(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)
	cmpOp := treecmp.MakeComparisonOperator(treecmp.GE)
	var input colexecop.Operator
	colIdx := 3
	inputTypes := make([]*types.T, colIdx+1)
	inputTypes[colIdx] = types.Int
	constVal := int16(31)
	constArg := tree.NewDInt(tree.DInt(constVal))
	outputIdx := 5
	calledOnNullInput := false
	op, err := GetProjectionRConstOperator(
		nil /* allocator */, inputTypes, types.Int2, types.Int, cmpOp, input, colIdx,
		constArg, outputIdx, nil /* EvalCtx */, nil /* BinFn */, nil /* cmpExpr */, calledOnNullInput,
	)
	if err != nil {
		t.Error(err)
	}
	expected := &projGEInt64Int16ConstOp{
		projConstOpBase: projConstOpBase{
			OneInputHelper:    colexecop.MakeOneInputHelper(op.(*projGEInt64Int16ConstOp).Input),
			colIdx:            colIdx,
			outputIdx:         outputIdx,
			calledOnNullInput: calledOnNullInput,
		},
		constArg: constVal,
	}
	if !reflect.DeepEqual(op, expected) {
		t.Errorf("got %+v,\nexpected %+v", op, expected)
	}
}
