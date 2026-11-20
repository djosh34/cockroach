// Copyright 2019 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package opt

import (
	"sort"

	"github.com/cockroachdb/cockroach/pkg/sql/opt/cat"
	"github.com/cockroachdb/cockroach/pkg/util/intsets"
)

// SchemaDeps contains information about the dependencies of objects in a
// schema, like a view or function.
type SchemaDeps []SchemaDep

// SchemaDep contains information about a dependency from a schema object
// (typically view and function) to a datasource.
type SchemaDep struct {
	DataSource cat.DataSource

	// ColumnOrdinals is the set of column ordinals that are referenced for this
	// table.
	ColumnOrdinals intsets.Fast

	// ColumnIDToOrd maps a scopeColumn's ColumnID to its ColumnOrdinal. This
	// helps us add only the columns that are actually referenced by the object's
	// query into the dependencies. We add a dependency on a column only when the
	// column is referenced and created as a scopeColumn.
	ColumnIDToOrd map[ColumnID]int

	// If an index is referenced specifically (via an index hint), SpecificIndex
	// is true and Index is the ordinal of that index.
	SpecificIndex bool
	Index         cat.IndexOrdinal
}

// SchemaTypeDeps contains a set of the IDs of types that
// this object depends on.
type SchemaTypeDeps = intsets.Fast

// GetColumnNames returns a sorted list of the names of the column dependencies
// and a boolean to determine if the dependency was a table.
// We only track column dependencies on tables.
func (dep SchemaDep) GetColumnNames() ([]string, bool) {
	colNames := make([]string, 0)
	if table, ok := dep.DataSource.(cat.Table); ok {
		dep.ColumnOrdinals.ForEach(func(i int) {
			name := table.Column(i).ColName()
			colNames = append(colNames, name.String())
		})
		sort.Strings(colNames)
		return colNames, ok
	}

	return nil, false
}
