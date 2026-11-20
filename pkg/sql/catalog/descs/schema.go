// Copyright 2021 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package descs

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/kv"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/catconstants"
)

// InsertDescriptorlessPublicSchemaToBatch adds the creation of a new descriptorless public
// schema to the batch.
func (tc *Collection) InsertDescriptorlessPublicSchemaToBatch(
	ctx context.Context, kvTrace bool, db catalog.DatabaseDescriptor, b *kv.Batch,
) error {
	return tc.InsertTempSchemaToBatch(ctx, kvTrace, db, catconstants.PublicSchemaName, keys.PublicSchemaID, b)
}

// DeleteDescriptorlessPublicSchemaToBatch adds the deletion of a new descriptorless public
// schema to the batch.
func (tc *Collection) DeleteDescriptorlessPublicSchemaToBatch(
	ctx context.Context, kvTrace bool, db catalog.DatabaseDescriptor, b *kv.Batch,
) error {
	return tc.DeleteTempSchemaToBatch(ctx, kvTrace, db, catconstants.PublicSchemaName, b)
}

// InsertTempSchemaToBatch adds the creation of a new temporary schema to
// the batch.
func (tc *Collection) InsertTempSchemaToBatch(
	ctx context.Context,
	kvTrace bool,
	db catalog.DatabaseDescriptor,
	tempSchemaName string,
	tempSchemaID descpb.ID,
	b *kv.Batch,
) error {
	entry := tempSchemaNameEntry{
		NameInfo: descpb.NameInfo{
			ParentID: db.GetID(),
			Name:     tempSchemaName,
		},
		id: tempSchemaID,
	}
	return tc.InsertNamespaceEntryToBatch(ctx, kvTrace, &entry, b)
}

// DeleteTempSchemaToBatch adds the deletion of a temporary schema to the
// batch.
func (tc *Collection) DeleteTempSchemaToBatch(
	ctx context.Context,
	kvTrace bool,
	db catalog.DatabaseDescriptor,
	tempSchemaName string,
	b *kv.Batch,
) error {
	nameInfo := descpb.NameInfo{
		ParentID: db.GetID(),
		Name:     tempSchemaName,
	}
	return tc.DeleteNamespaceEntryToBatch(ctx, kvTrace, &nameInfo, b)
}

type tempSchemaNameEntry struct {
	descpb.NameInfo
	id descpb.ID
}

var _ catalog.NameEntry = &tempSchemaNameEntry{}

// GetID is part of the catalog.NameEntry interface.
func (t tempSchemaNameEntry) GetID() descpb.ID {
	return t.id
}
