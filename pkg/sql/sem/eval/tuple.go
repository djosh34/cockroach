// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package eval

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree/treecmp"
)

// PickFromTuple picks the greatest (or least value) from a tuple.
func PickFromTuple(
	ctx context.Context, evalCtx *Context, greatest bool, args tree.Datums,
) (tree.Datum, error) {
	g := args[0]
	// Pick a greater (or smaller) value.
	for _, d := range args[1:] {
		var eval tree.Datum
		var err error
		if greatest {
			eval, err = evalComparison(ctx, evalCtx, treecmp.MakeComparisonOperator(treecmp.LT), g, d)
		} else {
			eval, err = evalComparison(ctx, evalCtx, treecmp.MakeComparisonOperator(treecmp.LT), d, g)
		}
		if err != nil {
			return nil, err
		}
		if eval == tree.DBoolTrue ||
			(eval == tree.DNull && g == tree.DNull) {
			g = d
		}
	}
	return g, nil
}
