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
	"github.com/cockroachdb/cockroach/pkg/clusterversion"
	"github.com/cockroachdb/cockroach/pkg/sql/schemachanger/scop"
	"github.com/cockroachdb/cockroach/pkg/sql/schemachanger/scpb"
)

func init() {

	opRegistry.register((*scpb.Sequence)(nil),
		toPublic(
			scpb.Status_ABSENT,
			equiv(scpb.Status_DROPPED),
			to(scpb.Status_DESCRIPTOR_ADDED,
				emit(func(this *scpb.Sequence) *scop.CreateSequenceDescriptor {
					return &scop.CreateSequenceDescriptor{
						SequenceID: this.SequenceID,
					}
				}),
			),
			to(scpb.Status_PUBLIC,
				revertible(false),
				emit(func(this *scpb.Sequence) *scop.InitSequence {
					return &scop.InitSequence{
						SequenceID:     this.SequenceID,
						RestartWith:    this.RestartWith,
						UseRestartWith: this.UseRestartWith,
					}
				}),
				emit(func(this *scpb.Sequence) *scop.MarkDescriptorAsPublic {
					return &scop.MarkDescriptorAsPublic{
						DescriptorID: this.SequenceID,
					}
				}),
			),
		),
		toAbsent(scpb.Status_PUBLIC,
			equiv(scpb.Status_DESCRIPTOR_ADDED),
			to(scpb.Status_DROPPED,
				revertible(false),
				emit(func(this *scpb.Sequence) *scop.MarkDescriptorAsDropped {
					return &scop.MarkDescriptorAsDropped{
						DescriptorID: this.SequenceID,
					}
				}),
			),
			to(scpb.Status_ABSENT,
				emit(func(this *scpb.Sequence, md *opGenContext) *scop.CreateGCJobForTable {
					if !md.ActiveVersion.IsActive(clusterversion.V23_1) {
						return &scop.CreateGCJobForTable{
							TableID:             this.SequenceID,
							DatabaseID:          databaseIDFromDroppedNamespaceTarget(md, this.SequenceID),
							StatementForDropJob: statementForDropJob(this, md),
						}
					}
					return nil
				}),
			),
		),
	)
}
