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

	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
	"github.com/cockroachdb/cockroach/pkg/sql/privilege"
	"github.com/cockroachdb/cockroach/pkg/sql/sqlerrors"
	"github.com/cockroachdb/errors"
)

// checkCanCreateOnSchema is a variant of (*planner).checkCanCreateOnSchema that
// avoids looking up descriptors -- we already have them.
func (g *testSchemaGenerator) checkCanCreateOnSchema(
	ctx context.Context, db catalog.DatabaseDescriptor, sc catalog.SchemaDescriptor,
) {
	switch sc.SchemaKind() {
	case catalog.SchemaPublic:
		// The public schema is valid to create in if the parent database is.
		if err := g.ext.cat.CheckPrivilegeForUser(ctx, db, privilege.CREATE, g.cfg.user); err != nil {
			panic(genError{err})
		}

	case catalog.SchemaTemporary:
		// The temp schema can always be written to after it's been
		// created.

	case catalog.SchemaVirtual:
		panic(genError{sqlerrors.NewCannotModifyVirtualSchemaError(sc.GetName())})

	case catalog.SchemaUserDefined:
		if err := g.ext.cat.CheckPrivilegeForUser(ctx, sc, privilege.CREATE, g.cfg.user); err != nil {
			panic(genError{err})
		}

	default:
		panic(genError{errors.AssertionFailedf("unknown schema kind %d", sc.SchemaKind())})
	}
}

func (g *testSchemaGenerator) checkCanCreateOnDatabase(
	ctx context.Context, db catalog.DatabaseDescriptor,
) {
	if err := g.ext.cat.CheckPrivilegeForUser(ctx, db, privilege.CREATE, g.cfg.user); err != nil {
		panic(genError{err})
	}
}

func (g *testSchemaGenerator) checkCanCreateDatabase(ctx context.Context) {
	if err := g.ext.cat.CanCreateDatabase(ctx); err != nil {
		panic(genError{err})
	}
}
