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

import (
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree/treebin"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
	"github.com/cockroachdb/cockroach/pkg/util/iterutil"
)

// InferBinaryType infers the return type of a binary expression, given the type
// of its inputs.
func InferBinaryType(bin treebin.BinaryOperatorSymbol, leftType, rightType *types.T) *types.T {
	o, ok := FindBinaryOverload(bin, leftType, rightType)
	if !ok {
		return nil
	}
	return o.ReturnType
}

// FindBinaryOverload finds the correct type signature overload for the
// specified binary operator, given the types of its inputs. If an overload is
// found, FindBinaryOverload returns true, plus a pointer to the overload.
// If an overload is not found, FindBinaryOverload returns false.
func FindBinaryOverload(
	bin treebin.BinaryOperatorSymbol, leftType, rightType *types.T,
) (ret *BinOp, ok bool) {

	// Find the binary op that matches the type of the expression's left and
	// right children. No more than one match should ever be found. The
	// TestTypingBinaryAssumptions test ensures this will be the case even if
	// new operators or overloads are added.
	_ = BinOps[bin].ForEachBinOp(func(o *BinOp) error {
		if leftType.Family() == types.UnknownFamily {
			ok = rightType.Equivalent(o.RightType)
		} else if rightType.Family() == types.UnknownFamily {
			ok = leftType.Equivalent(o.LeftType)
		} else {
			ok = leftType.Equivalent(o.LeftType) && rightType.Equivalent(o.RightType)
		}
		if !ok {
			return nil
		}
		ret = o
		return iterutil.StopIteration()
	})
	return ret, ok
}
