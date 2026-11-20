// Copyright 2022 The Cockroach Authors.
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

	"github.com/cockroachdb/cockroach/pkg/sql/catalog/catalogkeys"
	"github.com/cockroachdb/cockroach/pkg/sql/schemachanger/scop"
)

func (i *immediateVisitor) UpsertDatabaseComment(
	_ context.Context, op scop.UpsertDatabaseComment,
) error {
	i.AddComment(op.DatabaseID, 0, catalogkeys.DatabaseCommentType, op.Comment)
	return nil
}

func (i *immediateVisitor) UpsertSchemaComment(
	_ context.Context, op scop.UpsertSchemaComment,
) error {
	i.AddComment(op.SchemaID, 0, catalogkeys.SchemaCommentType, op.Comment)
	return nil
}

func (i *immediateVisitor) UpsertTableComment(_ context.Context, op scop.UpsertTableComment) error {
	i.AddComment(op.TableID, 0, catalogkeys.TableCommentType, op.Comment)
	return nil
}

func (i *immediateVisitor) UpsertColumnComment(
	_ context.Context, op scop.UpsertColumnComment,
) error {
	subID := int(op.ColumnID)
	if op.PGAttributeNum != 0 {
		// PGAttributeNum is only set if it differs from the ColumnID.
		subID = int(op.PGAttributeNum)
	}
	i.AddComment(op.TableID, subID, catalogkeys.ColumnCommentType, op.Comment)
	return nil
}

func (i *immediateVisitor) UpsertIndexComment(_ context.Context, op scop.UpsertIndexComment) error {
	i.AddComment(op.TableID, int(op.IndexID), catalogkeys.IndexCommentType, op.Comment)
	return nil
}

func (i *immediateVisitor) UpsertConstraintComment(
	_ context.Context, op scop.UpsertConstraintComment,
) error {
	i.AddComment(op.TableID, int(op.ConstraintID), catalogkeys.ConstraintCommentType, op.Comment)
	return nil
}
