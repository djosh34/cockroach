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

	"github.com/cockroachdb/cockroach/pkg/kv"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/tabledesc"
)

// GetLeasedImmutableTableByID returns a leased immutable table descriptor by
// its ID.
func (tc *Collection) GetLeasedImmutableTableByID(
	ctx context.Context, txn *kv.Txn, tableID descpb.ID,
) (catalog.TableDescriptor, error) {
	desc, _, err := tc.leased.getByID(ctx, txn, tableID)
	if err != nil || desc == nil {
		return nil, err
	}
	descs := []catalog.Descriptor{desc}
	hydrateFlags := getterFlags{
		descFilters: descFilters{
			withoutDropped: true,
			withoutOffline: true,
		},
	}
	err = tc.hydrateDescriptors(ctx, txn, hydrateFlags, descs)
	if err != nil {
		return nil, err
	}
	return catalog.AsTableDescriptor(descs[0])
}

// GetUncommittedMutableTableByID returns an uncommitted mutable table by its
// ID.
func (tc *Collection) GetUncommittedMutableTableByID(
	id descpb.ID,
) (catalog.TableDescriptor, *tabledesc.Mutable, error) {
	original := tc.uncommitted.getOriginalByID(id)
	mut := tc.uncommitted.getUncommittedMutableByID(id)
	if mut == nil {
		return nil, nil, nil
	}
	if _, err := catalog.AsTableDescriptor(mut); err != nil {
		return nil, nil, err
	}
	if original == nil {
		return nil, mut.(*tabledesc.Mutable), nil
	}
	if _, err := catalog.AsTableDescriptor(original); err != nil {
		return nil, nil, err
	}
	return original.(catalog.TableDescriptor), mut.(*tabledesc.Mutable), nil
}
