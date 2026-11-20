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
	"github.com/cockroachdb/cockroach/pkg/sql/schemachanger/scop"
	"github.com/cockroachdb/cockroach/pkg/sql/schemachanger/scpb"
	"github.com/cockroachdb/cockroach/pkg/util/protoutil"
)

func init() {

	// TODO(ajwerner): At time of writing, we have not implemented any DDLs which
	// modify the partitioning of a table or index. The only case where
	// partitioning matters is when partitioning is applied to new indexes
	// created for the purpose of adding or dropping columns. Given that, there's
	// no need to implement rolling back.
	//
	// Note that for correctness, we'll apply the partitioning in the same stage
	// as the index entering DELETE_ONLY/BACKFILLING so that zone configurations
	// can be applied to indexes as they are being backfilled.
	opRegistry.register((*scpb.IndexPartitioning)(nil),
		toPublic(
			scpb.Status_ABSENT,
			to(scpb.Status_PUBLIC,
				emit(func(this *scpb.IndexPartitioning) *scop.AddIndexPartitionInfo {
					return &scop.AddIndexPartitionInfo{
						Partitioning: *protoutil.Clone(this).(*scpb.IndexPartitioning),
					}
				}),
			),
		),
		toTransientAbsentLikePublic(),
		toAbsent(
			scpb.Status_PUBLIC,
			to(scpb.Status_ABSENT),
		),
	)
}
