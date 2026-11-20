// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package sql

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
)

func (p *planner) updateFunctionReferencesForCheck(
	ctx context.Context, tblDesc catalog.TableDescriptor, ck *descpb.TableDescriptor_CheckConstraint,
) error {
	udfIDs, err := tblDesc.GetAllReferencedFunctionIDsInConstraint(ck.ConstraintID)
	if err != nil {
		return err
	}
	for _, id := range udfIDs.Ordered() {
		fnDesc, err := p.descCollection.MutableByID(p.txn).Function(ctx, id)
		if err != nil {
			return err
		}
		if err := fnDesc.AddConstraintReference(tblDesc.GetID(), ck.ConstraintID); err != nil {
			return err
		}
		if err := p.writeFuncSchemaChange(ctx, fnDesc); err != nil {
			return err
		}
	}
	return nil
}

func (p *planner) maybeUpdateFunctionReferencesForColumn(
	ctx context.Context, tblDesc catalog.TableDescriptor, col *descpb.ColumnDescriptor,
) error {
	// Remove back references in old referenced functions.
	for _, id := range col.UsesFunctionIds {
		fnDesc, err := p.descCollection.MutableByID(p.txn).Function(ctx, id)
		if err != nil {
			return err
		}
		fnDesc.RemoveColumnReference(tblDesc.GetID(), col.ID)
		if err := p.writeFuncSchemaChange(ctx, fnDesc); err != nil {
			return err
		}
	}

	udfIDs, err := tblDesc.GetAllReferencedFunctionIDsInColumnExprs(col.ID)
	if err != nil {
		return err
	}
	col.UsesFunctionIds = udfIDs.Ordered()

	// Add new back references.
	for _, id := range col.UsesFunctionIds {
		fnDesc, err := p.descCollection.MutableByID(p.txn).Function(ctx, id)
		if err != nil {
			return err
		}
		if err := fnDesc.AddColumnReference(tblDesc.GetID(), col.ID); err != nil {
			return err
		}
		if err := p.writeFuncSchemaChange(ctx, fnDesc); err != nil {
			return err
		}
	}
	return nil
}
