// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package release_22_2

import (
	"github.com/cockroachdb/cockroach/pkg/sql/schemachanger/rel"
	"github.com/cockroachdb/cockroach/pkg/sql/schemachanger/scpb"
	"github.com/cockroachdb/cockroach/pkg/sql/schemachanger/screl"
)

// FromHasPublicStatusIfFromIsTableAndToIsRowLevelTTL creates
// a clause which leads to the outer clause failing to unify
// if the passed element `from` is a Table, `to` is a RowLevelTTl,
// and there does not exist a Node with the same Target as
// `fromTarget` in PUBLIC status.
// It is used to suppress rule "descriptor drop right before dependent element removal"
// for the special case where we drop a rowLevelTTL table in mixed
// version state for forward compatibility (issue #86672).
var FromHasPublicStatusIfFromIsTableAndToIsRowLevelTTL = screl.Schema.DefNotJoin3(
	"fromHasPublicStatusIfFromIsTableAndToIsRowLevelTTL",
	"fromTarget", "fromEl", "toEl", func(fromTarget, fromEl, toEl rel.Var) rel.Clauses {
		n := rel.Var("n")
		return rel.Clauses{
			fromEl.Type((*scpb.Table)(nil)),
			toEl.Type((*scpb.RowLevelTTL)(nil)),
			n.Type((*screl.Node)(nil)),
			n.AttrEqVar(screl.Target, fromTarget),
			screl.Schema.DefNotJoin1("nodeHasNoPublicStatus", "n", func(n rel.Var) rel.Clauses {
				public := rel.Var("public")
				return rel.Clauses{
					public.Eq(scpb.Status_PUBLIC),
					n.AttrEqVar(screl.CurrentStatus, public),
				}
			})(n),
		}
	})
