// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package randgen

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/sql/catalog/dbdesc"
)

// maybeGenDatabases either creates some test databases or recurses to
// create schemas/tables in an existing database.
func (g *testSchemaGenerator) maybeGenDatabases(ctx context.Context) {
	if g.gencfg.createDatabases {
		g.checkCanCreateDatabase(ctx)
		g.genDatabases(ctx)
		return
	}

	db := g.target.db
	if g.gencfg.createSchemas {
		g.checkCanCreateOnDatabase(ctx, db)
	}
	g.maybeGenSchemas(ctx, db)
}

// maybeGenSchema either creates some test schemas or recurses
// to create test tables.
// The caller is responsible for checking the user has privilege
// to create schemas on the target database.
func (g *testSchemaGenerator) maybeGenSchemas(ctx context.Context, db *dbdesc.Mutable) {
	if g.gencfg.createSchemas {
		g.genSchemas(ctx, db)
		return
	}

	sc := g.target.sc
	if g.gencfg.createTables && !g.gencfg.useGeneratedPublicSchema {
		g.checkCanCreateOnSchema(ctx, db, sc)
	}
	g.genMultipleTables(ctx, db, sc)
}
