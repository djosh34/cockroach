// Copyright 2020 The Cockroach Authors.
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
	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/nstree"
)

type syntheticDescriptors struct {
	descs nstree.NameMap
}

func (sd *syntheticDescriptors) add(desc catalog.Descriptor) {
	if mut, ok := desc.(catalog.MutableDescriptor); ok {
		desc = mut.ImmutableCopy()
	}
	sd.descs.Upsert(desc, desc.SkipNamespace())
}

func (sd *syntheticDescriptors) reset() {
	sd.descs.Clear()
}

func (sd *syntheticDescriptors) getSyntheticByName(
	dbID descpb.ID, schemaID descpb.ID, name string,
) catalog.NameEntry {
	return sd.descs.GetByName(dbID, schemaID, name)
}

func (sd *syntheticDescriptors) getSyntheticByID(id descpb.ID) catalog.Descriptor {
	if entry := sd.descs.GetByID(id); entry != nil {
		return entry.(catalog.Descriptor)
	}
	return nil
}

// iterateSyntheticByID applies fn to the synthetic descriptors in ascending
// sequence of IDs.
func (sd *syntheticDescriptors) iterateSyntheticByID(fn func(desc catalog.Descriptor) error) error {
	return sd.descs.IterateByID(func(entry catalog.NameEntry) error {
		return fn(entry.(catalog.Descriptor))
	})
}
