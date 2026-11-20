// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package catkv

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/clusterversion"
	"github.com/cockroachdb/cockroach/pkg/kv"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/internal/validate"
)

// DescriptorValidationModeProvider encapsulates the descriptor_validation
// session variable state.
type DescriptorValidationModeProvider interface {

	// ValidateDescriptorsOnRead returns true iff 'on' or 'read_only'.
	ValidateDescriptorsOnRead() bool

	// ValidateDescriptorsOnWrite returns true iff 'on'.
	ValidateDescriptorsOnWrite() bool
}

// NewCatalogReaderBackedValidationDereferencer returns a
// validate.ValidationDereferencer implementation backed by a
// CatalogReader. It's usually preferable to use a cached CatalogReader
// implementation for this purpose.
func NewCatalogReaderBackedValidationDereferencer(
	cr CatalogReader, txn *kv.Txn, dvmpMaybe DescriptorValidationModeProvider,
) validate.ValidationDereferencer {
	return &catalogReaderBackedDereferencer{
		cr:   cr,
		dvmp: dvmpMaybe,
		txn:  txn,
	}
}

type catalogReaderBackedDereferencer struct {
	cr   CatalogReader
	dvmp DescriptorValidationModeProvider
	txn  *kv.Txn
}

var _ validate.ValidationDereferencer = &catalogReaderBackedDereferencer{}

// DereferenceDescriptors implements the validate.ValidationDereferencer
// interface.
func (c catalogReaderBackedDereferencer) DereferenceDescriptors(
	ctx context.Context, version clusterversion.ClusterVersion, reqs []descpb.ID,
) ([]catalog.Descriptor, error) {
	const isDescriptorRequired = false
	read, err := c.cr.GetByIDs(ctx, c.txn, reqs, isDescriptorRequired, catalog.Any)
	if err != nil {
		return nil, err
	}
	ret := make([]catalog.Descriptor, len(reqs))
	for i, id := range reqs {
		desc := read.LookupDescriptor(id)
		if desc == nil {
			continue
		}
		ret[i] = desc
		if c.dvmp != nil && !c.dvmp.ValidateDescriptorsOnRead() {
			continue
		}
		if err = validate.Self(version, desc); err != nil {
			return nil, err
		}
	}
	return ret, nil
}

// DereferenceDescriptorIDs implements the validate.ValidationDereferencer
// interface.
func (c catalogReaderBackedDereferencer) DereferenceDescriptorIDs(
	ctx context.Context, reqs []descpb.NameInfo,
) ([]descpb.ID, error) {
	read, err := c.cr.GetByNames(ctx, c.txn, reqs)
	if err != nil {
		return nil, err
	}
	ret := make([]descpb.ID, len(reqs))
	for i, nameInfo := range reqs {
		if ne := read.LookupNamespaceEntry(nameInfo); ne != nil {
			ret[i] = ne.GetID()
		}
	}
	return ret, nil
}
