// Copyright 2020 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package testcat

import (
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/errors"
)

// CreateIndex is a partial implementation of the CREATE INDEX statement.
func (tc *Catalog) CreateIndex(stmt *tree.CreateIndex, version descpb.IndexDescriptorVersion) {
	tn := stmt.Table
	// Update the table name to include catalog and schema if not provided.
	tc.qualifyTableName(&tn)
	tab, err := tc.LookupTable(&tn)
	var view *View
	if err == nil {
		for _, idx := range tab.Indexes {
			in := stmt.Name.String()
			if idx.IdxName == in {
				panic(errors.Newf(`relation "%s" already exists`, in))
			}
		}
	} else {
		view = tc.View(&tn)
	}

	// Convert stmt to a tree.IndexTableDef so that Table.addIndex can be used
	// to add the index to the table.
	indexTableDef := &tree.IndexTableDef{
		Name:         stmt.Name,
		Columns:      stmt.Columns,
		Sharded:      stmt.Sharded,
		Storing:      stmt.Storing,
		Inverted:     stmt.Inverted,
		Predicate:    stmt.Predicate,
		Invisibility: stmt.Invisibility,
	}

	idxType := nonUniqueIndex
	if stmt.Unique {
		idxType = uniqueIndex

	}
	if tab != nil {
		tab.addIndexWithVersion(indexTableDef, idxType, version)
	} else if view != nil {
		view.addIndex(indexTableDef)
	}
}
