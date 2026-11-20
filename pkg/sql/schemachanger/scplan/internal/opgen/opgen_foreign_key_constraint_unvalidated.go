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
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/schemachanger/scop"
	"github.com/cockroachdb/cockroach/pkg/sql/schemachanger/scpb"
)

func init() {
	opRegistry.register((*scpb.ForeignKeyConstraintUnvalidated)(nil),
		toPublic(
			scpb.Status_ABSENT,
			to(scpb.Status_PUBLIC,
				emit(func(this *scpb.ForeignKeyConstraintUnvalidated) *scop.AddForeignKeyConstraint {
					return &scop.AddForeignKeyConstraint{
						TableID:                 this.TableID,
						ConstraintID:            this.ConstraintID,
						ColumnIDs:               this.ColumnIDs,
						ReferencedTableID:       this.ReferencedTableID,
						ReferencedColumnIDs:     this.ReferencedColumnIDs,
						OnUpdateAction:          this.OnUpdateAction,
						OnDeleteAction:          this.OnDeleteAction,
						CompositeKeyMatchMethod: this.CompositeKeyMatchMethod,
						Validity:                descpb.ConstraintValidity_Unvalidated,
					}
				}),
			),
		),
		toAbsent(
			scpb.Status_PUBLIC,
			to(scpb.Status_ABSENT,
				emit(func(this *scpb.ForeignKeyConstraintUnvalidated) *scop.RemoveForeignKeyBackReference {
					return &scop.RemoveForeignKeyBackReference{
						ReferencedTableID:  this.ReferencedTableID,
						OriginTableID:      this.TableID,
						OriginConstraintID: this.ConstraintID,
					}
				}),
				emit(func(this *scpb.ForeignKeyConstraintUnvalidated) *scop.RemoveForeignKeyConstraint {
					return &scop.RemoveForeignKeyConstraint{
						TableID:      this.TableID,
						ConstraintID: this.ConstraintID,
					}
				}),
			),
		),
	)
}
