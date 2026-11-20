// Copyright 2022 The Cockroach Authors.
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
	"github.com/cockroachdb/cockroach/pkg/clusterversion"
	"github.com/cockroachdb/cockroach/pkg/sql/schemachanger/scop"
	"github.com/cockroachdb/cockroach/pkg/sql/schemachanger/scpb"
)

func init() {
	opRegistry.register(
		(*scpb.ColumnFamily)(nil),
		toPublic(
			scpb.Status_ABSENT,
			to(scpb.Status_PUBLIC,
				emit(func(this *scpb.ColumnFamily) *scop.AddColumnFamily {
					return &scop.AddColumnFamily{
						TableID:  this.TableID,
						FamilyID: this.FamilyID,
						Name:     this.Name,
					}
				}),
			),
		),
		toAbsent(
			scpb.Status_PUBLIC,
			to(scpb.Status_ABSENT,
				revertible(false),
				emit(func(this *scpb.ColumnFamily, md *opGenContext) *scop.AssertColumnFamilyIsRemoved {
					// Use stricter criteria on 23.1, which will guarantee that the
					// column family is cleaned up first using the column type element.
					// The only purpose this operation serves is to make sure that this
					// when the element reaches absent the column family has actually
					// been removed. This would have been done via the last column
					// type element referencing it.
					if md.ActiveVersion.IsActive(clusterversion.V23_1) {
						return &scop.AssertColumnFamilyIsRemoved{
							TableID:  this.TableID,
							FamilyID: this.FamilyID,
						}
					}
					// We would have used NotImplemented before, but that will be an
					// assertion now, so just return no-ops.
					return nil
				}),
			),
		),
	)
}
