// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package scmutationexec

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/sql/catalog/catpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/schemadesc"
	"github.com/cockroachdb/cockroach/pkg/sql/schemachanger/scop"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/catid"
)

func (i *immediateVisitor) CreateSchemaDescriptor(
	ctx context.Context, op scop.CreateSchemaDescriptor,
) error {
	mut := schemadesc.NewBuilder(&descpb.SchemaDescriptor{
		ParentID:   catid.InvalidDescID, // Set by `SchemaParent` element
		Name:       "",                  // Set by `Namespace` element
		ID:         op.SchemaID,
		Privileges: &catpb.PrivilegeDescriptor{Version: catpb.Version23_2}, // Populated by `UserPrivileges` elements and `Owner` element
		Version:    1,
	}).BuildCreatedMutableSchema()
	mut.State = descpb.DescriptorState_ADD
	i.CreateDescriptor(mut)
	return nil
}
