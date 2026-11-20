// Copyright 2021 The Cockroach Authors.
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
	"github.com/cockroachdb/cockroach/pkg/sql/schemachanger/scop"
	"github.com/cockroachdb/cockroach/pkg/sql/schemachanger/scpb"
	"github.com/cockroachdb/cockroach/pkg/util/protoutil"
)

func init() {
	opRegistry.register((*scpb.Column)(nil),
		toPublic(
			scpb.Status_ABSENT,
			to(scpb.Status_DELETE_ONLY,
				emit(func(this *scpb.Column) *scop.MakeAbsentColumnDeleteOnly {
					return &scop.MakeAbsentColumnDeleteOnly{
						Column: *protoutil.Clone(this).(*scpb.Column),
					}
				}),
			),
			to(scpb.Status_WRITE_ONLY,
				emit(func(this *scpb.Column) *scop.MakeDeleteOnlyColumnWriteOnly {
					return &scop.MakeDeleteOnlyColumnWriteOnly{
						TableID:  this.TableID,
						ColumnID: this.ColumnID,
					}
				}),
			),
			to(scpb.Status_PUBLIC,
				revertible(false),
				emit(func(this *scpb.Column, md *opGenContext) *scop.MakeWriteOnlyColumnPublic {
					return &scop.MakeWriteOnlyColumnPublic{
						TableID:  this.TableID,
						ColumnID: this.ColumnID,
					}
				}),
				emit(func(this *scpb.Column) *scop.RefreshStats {
					return &scop.RefreshStats{
						TableID: this.TableID,
					}
				}),
			),
		),
		toAbsent(
			scpb.Status_PUBLIC,
			to(scpb.Status_WRITE_ONLY,
				emit(func(this *scpb.Column) *scop.MakePublicColumnWriteOnly {
					return &scop.MakePublicColumnWriteOnly{
						TableID:  this.TableID,
						ColumnID: this.ColumnID,
					}
				}),
			),
			to(scpb.Status_DELETE_ONLY,
				revertible(false),
				emit(func(this *scpb.Column) *scop.MakeWriteOnlyColumnDeleteOnly {
					return &scop.MakeWriteOnlyColumnDeleteOnly{
						TableID:  this.TableID,
						ColumnID: this.ColumnID,
					}
				}),
			),
			to(scpb.Status_ABSENT,
				emit(func(this *scpb.Column, md *opGenContext) *scop.MakeDeleteOnlyColumnAbsent {
					return &scop.MakeDeleteOnlyColumnAbsent{
						TableID:  this.TableID,
						ColumnID: this.ColumnID,
					}
				}),
			),
		),
	)
}
