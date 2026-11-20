// Copyright 2017 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

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
