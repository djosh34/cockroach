// Copyright 2017 The Cockroach Authors.
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
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/catconstants"
)

// pgExtension is the virtual schema for the tables and views of postgres
// extensions. Postgres extensions typically install these on the public
// schema; we use this schema instead. It held the PostGIS tables, which are
// removed, so it is empty. It stays on the search path, as in PostgreSQL
// setups that install extensions into their own schema.
var pgExtension = virtualSchema{
	name:                       catconstants.PgExtensionSchemaName,
	tableDefs:                  map[descpb.ID]virtualSchemaDef{},
	validWithNoDatabaseContext: false,
}
