// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package opgen

import (
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/catpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/schemachanger/scop"
	"github.com/cockroachdb/cockroach/pkg/sql/schemachanger/scpb"
)

func init() {
	opRegistry.register((*scpb.UniqueWithoutIndexConstraintUnvalidated)(nil),
		toPublic(
			scpb.Status_ABSENT,
			to(scpb.Status_PUBLIC,
				emit(func(this *scpb.UniqueWithoutIndexConstraintUnvalidated) *scop.AddUniqueWithoutIndexConstraint {
					var partialExpr catpb.Expression
					if this.Predicate != nil {
						partialExpr = this.Predicate.Expr
					}
					return &scop.AddUniqueWithoutIndexConstraint{
						TableID:      this.TableID,
						ConstraintID: this.ConstraintID,
						ColumnIDs:    this.ColumnIDs,
						PartialExpr:  partialExpr,
						Validity:     descpb.ConstraintValidity_Unvalidated,
					}
				}),
				emit(func(this *scpb.UniqueWithoutIndexConstraintUnvalidated) *scop.UpdateTableBackReferencesInTypes {
					if this.Predicate == nil || this.Predicate.UsesTypeIDs == nil || len(this.Predicate.UsesTypeIDs) == 0 {
						return nil
					}
					return &scop.UpdateTableBackReferencesInTypes{
						TypeIDs:               this.Predicate.UsesTypeIDs,
						BackReferencedTableID: this.TableID,
					}
				}),
				emit(func(this *scpb.UniqueWithoutIndexConstraintUnvalidated) *scop.UpdateTableBackReferencesInSequences {
					if this.Predicate == nil || this.Predicate.UsesSequenceIDs == nil || len(this.Predicate.UsesSequenceIDs) == 0 {
						return nil
					}
					return &scop.UpdateTableBackReferencesInSequences{
						SequenceIDs:           this.Predicate.UsesSequenceIDs,
						BackReferencedTableID: this.TableID,
					}
				}),
			),
		),
		toAbsent(
			scpb.Status_PUBLIC,
			to(scpb.Status_ABSENT,
				emit(func(this *scpb.UniqueWithoutIndexConstraintUnvalidated) *scop.RemoveUniqueWithoutIndexConstraint {
					return &scop.RemoveUniqueWithoutIndexConstraint{
						TableID:      this.TableID,
						ConstraintID: this.ConstraintID,
					}
				}),
				emit(func(this *scpb.UniqueWithoutIndexConstraintUnvalidated) *scop.UpdateTableBackReferencesInTypes {
					if this.Predicate == nil || this.Predicate.UsesTypeIDs == nil || len(this.Predicate.UsesTypeIDs) == 0 {
						return nil
					}
					return &scop.UpdateTableBackReferencesInTypes{
						TypeIDs:               this.Predicate.UsesTypeIDs,
						BackReferencedTableID: this.TableID,
					}
				}),
				emit(func(this *scpb.UniqueWithoutIndexConstraintUnvalidated) *scop.UpdateTableBackReferencesInSequences {
					if this.Predicate == nil || this.Predicate.UsesSequenceIDs == nil || len(this.Predicate.UsesSequenceIDs) == 0 {
						return nil
					}
					return &scop.UpdateTableBackReferencesInSequences{
						SequenceIDs:           this.Predicate.UsesSequenceIDs,
						BackReferencedTableID: this.TableID,
					}
				}),
			),
		),
	)
}
