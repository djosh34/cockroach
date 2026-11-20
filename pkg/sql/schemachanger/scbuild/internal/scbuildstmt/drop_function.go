// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package scbuildstmt

import (
	"strings"

	"github.com/cockroachdb/cockroach/pkg/sql/pgwire/pgcode"
	"github.com/cockroachdb/cockroach/pkg/sql/pgwire/pgerror"
	"github.com/cockroachdb/cockroach/pkg/sql/schemachanger/scerrors"
	"github.com/cockroachdb/cockroach/pkg/sql/schemachanger/scpb"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/catid"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
)

func DropFunction(b BuildCtx, n *tree.DropRoutine) {
	if n.DropBehavior == tree.DropCascade {
		// TODO(chengxiong): remove this when we allow UDF usage.
		panic(scerrors.NotImplementedErrorf(n, "cascade dropping functions"))
	}

	routineType := tree.UDFRoutine
	if n.Procedure {
		routineType = tree.ProcedureRoutine
	}

	var toCheckBackRefs []catid.DescID
	var toCheckBackRefsNames []*scpb.FunctionName
	for _, f := range n.Routines {
		elts := b.ResolveRoutine(&f, ResolveParams{
			IsExistenceOptional: n.IfExists,
		}, routineType)
		_, _, fn := scpb.FindFunction(elts)
		if fn == nil {
			continue
		}
		f.FuncName.ObjectNamePrefix = b.NamePrefix(fn)
		if dropRestrictDescriptor(b, fn.FunctionID) {
			toCheckBackRefs = append(toCheckBackRefs, fn.FunctionID)
			_, _, fnName := scpb.FindFunctionName(elts)
			toCheckBackRefsNames = append(toCheckBackRefsNames, fnName)
		}
		b.LogEventForExistingTarget(fn)
		b.IncrementSubWorkID()
		b.IncrementSchemaChangeDropCounter("function")
	}

	for i, fnID := range toCheckBackRefs {
		dependentNames := dependentTypeNames(b, fnID)
		if len(dependentNames) > 0 {
			panic(pgerror.Newf(
				pgcode.DependentObjectsStillExist,
				"cannot drop function %q because other objects ([%v]) still depend on it",
				toCheckBackRefsNames[i].Name, strings.Join(dependentNames, ", "),
			))

		}
	}
}
