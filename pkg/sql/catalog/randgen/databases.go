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
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/schemadesc"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/catconstants"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/catid"
	"github.com/cockroachdb/cockroach/pkg/util/randident"
	"github.com/cockroachdb/errors"
)

func (g *testSchemaGenerator) genDatabases(ctx context.Context) {
	if g.gencfg.numDatabases == 0 {
		return
	}

	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(genError); ok {
				e.err = errors.Wrapf(e.err,
					"generating %d databases with name pattern %q",
					g.gencfg.numDatabases, g.gencfg.dbNamePat)
				panic(e)
			}
			panic(r)
		}
	}()

	// Compute the names ahead of time; this also takes care of
	// avoiding duplicates.
	ng := randident.NewNameGenerator(&g.cfg.NameGen, g.rand, g.gencfg.dbNamePat)
	conflictNames := g.dbNames(ctx)
	dbNames, err := ng.GenerateMultiple(ctx, g.gencfg.numDatabases, conflictNames)
	if err != nil {
		panic(genError{err})
	}

	// Actually generate the databases.
	firstID := g.makeIDs(ctx, g.gencfg.numDatabases*2)
	for i := 0; i < g.gencfg.numDatabases; i++ {
		dbName := dbNames[i]
		id := firstID + catid.DescID(i*2)
		publicSchemaID := firstID + catid.DescID(i*2+1)

		g.models.db.ID = id
		g.models.db.Name = dbName
		g.models.db.Schemas[catconstants.PublicSchemaName] = descpb.DatabaseDescriptor_SchemaInfo{ID: publicSchemaID}
		db := dbdesc.NewBuilder(&g.models.db).BuildCreatedMutableDatabase()
		g.cfg.GeneratedCounts.Databases++
		g.newDesc(ctx, db)

		g.models.publicSc.ParentID = id
		g.models.publicSc.ID = publicSchemaID
		publicSchema := schemadesc.NewBuilder(&g.models.publicSc).BuildCreatedMutableSchema()
		g.cfg.GeneratedCounts.Schemas++
		g.newDesc(ctx, publicSchema)

		g.target.sc = publicSchema
		g.maybeGenSchemas(ctx, db)
	}
}
