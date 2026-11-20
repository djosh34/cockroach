// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package funcdesc

import (
	"github.com/cockroachdb/cockroach/pkg/clusterversion"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/util/errorutil/unimplemented"
)

var schemaExprContextAllowingUDF = map[tree.SchemaExprContext]clusterversion.Key{
	tree.CheckConstraintExpr:           clusterversion.V23_1,
	tree.ColumnDefaultExprInNewTable:   clusterversion.V23_1,
	tree.ColumnDefaultExprInSetDefault: clusterversion.V23_1,
}

// MaybeFailOnUDFUsage returns an error if the given expression or any
// sub-expression used a UDF unless it's explicitly listed as an allowed use
// case.
// TODO(chengxiong): remove this function when we start allowing UDF references.
func MaybeFailOnUDFUsage(
	expr tree.TypedExpr, exprContext tree.SchemaExprContext, version clusterversion.ClusterVersion,
) error {
	if supportedVersion, ok := schemaExprContextAllowingUDF[exprContext]; ok && version.IsActive(supportedVersion) {
		return nil
	}
	visitor := &tree.UDFDisallowanceVisitor{}
	tree.WalkExpr(visitor, expr)
	if visitor.FoundUDF {
		return unimplemented.NewWithIssue(83234, "usage of user-defined function from relations not supported")
	}
	return nil
}
