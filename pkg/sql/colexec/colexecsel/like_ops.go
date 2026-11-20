// Copyright 2021 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package colexecsel

import (
	"github.com/cockroachdb/cockroach/pkg/sql/colexec/colexeccmp"
	"github.com/cockroachdb/cockroach/pkg/sql/colexecop"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/eval"
	"github.com/cockroachdb/errors"
)

// GetLikeOperator returns a selection operator which applies the specified LIKE
// pattern, or NOT LIKE if the negate argument is true. The implementation
// varies depending on the complexity of the pattern.
func GetLikeOperator(
	evalCtx *eval.Context,
	input colexecop.Operator,
	colIdx int,
	pattern string,
	negate bool,
	caseInsensitive bool,
) (colexecop.Operator, error) {
	likeOpType, patterns, err := colexeccmp.GetLikeOperatorType(pattern, caseInsensitive)
	if err != nil {
		return nil, err
	}
	pat := patterns[0]
	base := selConstOpBase{
		OneInputHelper: colexecop.MakeOneInputHelper(input),
		colIdx:         colIdx,
	}
	switch likeOpType {
	case colexeccmp.LikeAlwaysMatch:
		// Use an empty prefix operator to get correct NULL behavior. We don't
		// need to pay attention to the case sensitivity here since the pattern
		// will always match anyway.
		return &selPrefixBytesBytesConstOp{
			selConstOpBase: base,
			constArg:       []byte{},
			negate:         negate,
		}, nil
	case colexeccmp.LikeConstant:
		if caseInsensitive {
			// We don't have an equivalent projection operator that would
			// convert the argument to capital letters, so for now we fall back
			// to the default comparison operator.
			return nil, errors.New("ILIKE and NOT ILIKE aren't supported with a constant pattern")
		}
		if negate {
			return &selNEBytesBytesConstOp{
				selConstOpBase: base,
				constArg:       pat,
			}, nil
		}
		return &selEQBytesBytesConstOp{
			selConstOpBase: base,
			constArg:       pat,
		}, nil
	case colexeccmp.LikeContains:
		return &selContainsBytesBytesConstOp{
			selConstOpBase:  base,
			constArg:        pat,
			negate:          negate,
			caseInsensitive: caseInsensitive,
		}, nil
	case colexeccmp.LikePrefix:
		return &selPrefixBytesBytesConstOp{
			selConstOpBase:  base,
			constArg:        pat,
			negate:          negate,
			caseInsensitive: caseInsensitive,
		}, nil
	case colexeccmp.LikeRegexp:
		re, err := eval.ConvertLikeToRegexp(evalCtx, string(patterns[0]), caseInsensitive, '\\')
		if err != nil {
			return nil, err
		}
		return &selRegexpBytesBytesConstOp{
			selConstOpBase: base,
			constArg:       re,
			negate:         negate,
		}, nil
	case colexeccmp.LikeSkeleton:
		return &selSkeletonBytesBytesConstOp{
			selConstOpBase:  base,
			constArg:        patterns,
			negate:          negate,
			caseInsensitive: caseInsensitive,
		}, nil
	case colexeccmp.LikeSuffix:
		return &selSuffixBytesBytesConstOp{
			selConstOpBase:  base,
			constArg:        pat,
			negate:          negate,
			caseInsensitive: caseInsensitive,
		}, nil
	default:
		return nil, errors.AssertionFailedf("unsupported like op type %d", likeOpType)
	}
}
