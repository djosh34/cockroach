// Copyright 2026 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package upstreamcheck

import (
	"fmt"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/sql/catalog/catpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/dbdesc"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/funcdesc"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/nstree"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/schemadesc"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/tabledesc"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/typedesc"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/stretchr/testify/require"
)

func TestExprProblems(t *testing.T) {
	defer leaktest.AfterTest(t)()
	for _, tc := range []struct {
		expr string
		want []string
	}{
		{"", nil},
		{"now()", nil},
		{"lower(name) || 'x'", nil},
		{"crdb_internal.cluster_id()", nil},
		{"pg_catalog.length(name) > 0", nil},
		{"[FUNCTION 100105](a)", nil},
		{"st_area(geom) > 1", []string{"calls the removed function st_area"}},
		{"st_x(a) + st_area(a) + st_x(b)", []string{"calls the removed functions st_area, st_x"}},
		{"crdb_internal.fips_ready()", []string{"calls the removed function crdb_internal.fips_ready"}},
		{"decrypt(a, b, 'aes')", []string{"calls the removed function decrypt"}},
		{"a:::GEOMETRY IS OF (GEOGRAPHY)", []string{"uses the removed types GEOGRAPHY, GEOMETRY"}},
		{"(", []string{`cannot be parsed: at or near "EOF": syntax error`}},
	} {
		t.Run(tc.expr, func(t *testing.T) {
			require.Equal(t, tc.want, exprProblems(tc.expr))
		})
	}
}

func TestStmtProblems(t *testing.T) {
	defer leaktest.AfterTest(t)()
	isMyFunc := func(n tree.RoutineName) bool {
		return n.Object() == "my_func" && (!n.ExplicitSchema || n.Schema() == "public")
	}
	isMyGeometry := func(n *tree.UnresolvedObjectName) bool {
		return n.NumParts > 1 && n.Object() == "geometry" && n.Schema() == "my"
	}
	for _, tc := range []struct {
		stmts string
		want  []string
	}{
		{"SELECT a, count(*) FROM t GROUP BY a", nil},
		{"SELECT my_func(a), public.my_func(b) FROM t", nil},
		{"SELECT other.my_func(a) FROM t", []string{"calls the removed function other.my_func"}},
		{"SELECT st_x(g) FROM t", []string{"calls the removed function st_x"}},
		{"SELECT a FROM (SELECT st_x(g) AS a FROM t) WHERE a IN (SELECT st_y(g) FROM u)",
			[]string{"calls the removed functions st_x, st_y"}},
		{"SELECT 1; SELECT st_x(g) FROM t;", []string{"calls the removed function st_x"}},
		// The tree package's visitors skip join conditions and window
		// definitions.
		{"SELECT 1 FROM t JOIN (SELECT 1) ON crdb_internal.fips_ready()",
			[]string{"calls the removed function crdb_internal.fips_ready"}},
		{"SELECT rank() OVER (ORDER BY st_x(g)) FROM t", []string{"calls the removed function st_x"}},
		{"SELECT a FROM t WHERE wkt::GEOMETRY IS NOT NULL AND b::geography[] IS NULL",
			[]string{"uses the removed types GEOGRAPHY, GEOMETRY"}},
		{"SELECT st_x(a::BOX2D) FROM t", []string{
			"calls the removed function st_x", "uses the removed type BOX2D"}},
		{"SELECT a::INT8, b::@100105, c::myschema.mytype FROM t", nil},
		// A spatial type name is removed unless it names a user-defined type.
		{"SELECT 1 FROM t JOIN u ON t.w::my.geometry IS NOT NULL WHERE t.w IS OF (defaultdb.my.geometry)", nil},
		{"SELECT 1 FROM t JOIN u ON t.w::public.geometry IS NOT NULL WHERE t.w IS OF (defaultdb.public.geography)",
			[]string{"uses the removed types GEOGRAPHY, GEOMETRY"}},
		{"SELECT a::pg_catalog.box2d, b::other.geometry FROM t", []string{"uses the removed types BOX2D, GEOMETRY"}},
		{"SELECT a::GEOMETRY FROM t", []string{"uses the removed type GEOMETRY"}},
		// A quoted name in other case never named a spatial type.
		{`SELECT a::my."Geometry", b::public."Geography" FROM t`, nil},
		{"SELECT a::public.geometry FROM t", []string{"uses the removed type GEOMETRY"}},
	} {
		t.Run(tc.stmts, func(t *testing.T) {
			require.Equal(t, tc.want, stmtProblems(tc.stmts, isMyFunc, isMyGeometry))
		})
	}
}

// TestCatalogProblems builds descriptors as upstream CockroachDB wrote them
// and checks what catalogProblems reports.
func TestCatalogProblems(t *testing.T) {
	defer leaktest.AfterTest(t)()
	geometry := &types.T{InternalType: types.InternalType{Family: types.GeometryFamily}}
	primary := descpb.IndexDescriptor{ID: 1, Name: "primary", KeyColumnIDs: []descpb.ColumnID{1}}
	var mc nstree.MutableCatalog
	mc.UpsertDescriptor(dbdesc.NewBuilder(&descpb.DatabaseDescriptor{
		ID: 100, Name: "shop",
	}).BuildImmutable())
	mc.UpsertDescriptor(schemadesc.NewBuilder(&descpb.SchemaDescriptor{
		ID: 101, Name: "public", ParentID: 100,
	}).BuildImmutable())
	mc.UpsertDescriptor(dbdesc.NewBuilder(&descpb.DatabaseDescriptor{
		ID: 110, Name: "world", RegionConfig: &descpb.DatabaseDescriptor_RegionConfig{},
	}).BuildImmutable())
	// A plain table that uses nothing cut.
	mc.UpsertDescriptor(tabledesc.NewBuilder(&descpb.TableDescriptor{
		ID: 102, Name: "customers", ParentID: 100, UnexposedParentSchemaID: 101,
		Columns: []descpb.ColumnDescriptor{
			{ID: 1, Name: "id", Type: types.Int},
			{ID: 2, Name: "created", Type: types.TimestampTZ, DefaultExpr: strPtr("now():::TIMESTAMPTZ")},
		},
		PrimaryIndex: primary,
		Checks:       []*descpb.TableDescriptor_CheckConstraint{{Name: "check_id", Expr: "id > 0:::INT8"}},
	}).BuildImmutable())
	mc.UpsertDescriptor(tabledesc.NewBuilder(&descpb.TableDescriptor{
		ID: 103, Name: "places", ParentID: 100, UnexposedParentSchemaID: 101,
		Columns: []descpb.ColumnDescriptor{
			{ID: 1, Name: "id", Type: types.Int},
			{ID: 2, Name: "geom", Type: geometry},
			{ID: 3, Name: "area", Type: types.Float, ComputeExpr: strPtr("st_area(geom)")},
		},
		PrimaryIndex: primary,
		Indexes: []descpb.IndexDescriptor{{
			ID: 2, Name: "places_geom_idx", KeyColumnIDs: []descpb.ColumnID{2},
			GeoConfig: catpb.GeoIndexConfig{S2Geometry: &catpb.S2GeometryConfig{}},
		}, {
			ID: 3, Name: "places_id_idx", KeyColumnIDs: []descpb.ColumnID{1},
			Partitioning: catpb.PartitioningDescriptor{NumColumns: 1},
			Predicate:    "st_x(geom) > 0",
		}},
		Checks: []*descpb.TableDescriptor_CheckConstraint{{Name: "check_geom", Expr: "st_isvalid(geom)"}},
		UniqueWithoutIndexConstraints: []descpb.UniqueWithoutIndexConstraint{{
			Name: "unique_area", ColumnIDs: []descpb.ColumnID{3}, Predicate: "st_y(geom) > 0",
		}},
		LocalityConfig: &catpb.LocalityConfig{},
	}).BuildImmutable())
	mc.UpsertDescriptor(tabledesc.NewBuilder(&descpb.TableDescriptor{
		ID: 104, Name: "areas", ParentID: 100, UnexposedParentSchemaID: 101,
		Columns:      []descpb.ColumnDescriptor{{ID: 1, Name: "area", Type: types.Float}},
		PrimaryIndex: primary,
		ViewQuery:    "SELECT st_area(geom) AS area FROM shop.public.places",
		State:        descpb.DescriptorState_DROP,
	}).BuildImmutable())
	mc.UpsertDescriptor(typedesc.NewBuilder(&descpb.TypeDescriptor{
		ID: 105, Name: "pair", ParentID: 100, ParentSchemaID: 101, Kind: descpb.TypeDescriptor_COMPOSITE,
		Composite: &descpb.TypeDescriptor_Composite{Elements: []descpb.TypeDescriptor_Composite_CompositeElement{
			{ElementLabel: "a", ElementType: types.Int},
			{ElementLabel: "g", ElementType: geometry},
		}},
	}).BuildImmutable())
	mc.UpsertDescriptor(typedesc.NewBuilder(&descpb.TypeDescriptor{
		ID: 111, Name: "crdb_internal_region", ParentID: 110, ParentSchemaID: 112,
		Kind: descpb.TypeDescriptor_MULTIREGION_ENUM,
	}).BuildImmutable())
	mc.UpsertDescriptor(funcdesc.NewBuilder(&descpb.FunctionDescriptor{
		ID: 106, Name: "my_func", ParentID: 100, ParentSchemaID: 101, Lang: catpb.Function_SQL,
		ReturnType:   descpb.FunctionDescriptor_ReturnType{Type: types.Int},
		FunctionBody: "SELECT 1;",
	}).BuildImmutable())
	mc.UpsertDescriptor(funcdesc.NewBuilder(&descpb.FunctionDescriptor{
		ID: 107, Name: "plain_func", ParentID: 100, ParentSchemaID: 101, Lang: catpb.Function_SQL,
		ReturnType:   descpb.FunctionDescriptor_ReturnType{Type: types.Int},
		FunctionBody: "SELECT length('x');",
	}).BuildImmutable())
	mc.UpsertDescriptor(funcdesc.NewBuilder(&descpb.FunctionDescriptor{
		ID: 108, Name: "plpgsql_func", ParentID: 100, ParentSchemaID: 101, Lang: catpb.Function_PLPGSQL,
		ReturnType:   descpb.FunctionDescriptor_ReturnType{Type: types.Int},
		FunctionBody: "BEGIN RETURN 1; END",
	}).BuildImmutable())
	mc.UpsertDescriptor(funcdesc.NewBuilder(&descpb.FunctionDescriptor{
		ID: 109, Name: "area", ParentID: 100, ParentSchemaID: 101, Lang: catpb.Function_SQL,
		Params:       []descpb.FunctionDescriptor_Parameter{{Name: "g", Type: geometry}},
		ReturnType:   descpb.FunctionDescriptor_ReturnType{Type: types.Float},
		FunctionBody: "SELECT st_area(g);",
	}).BuildImmutable())
	// A view records the functions it calls, and a routine can call none, so
	// a function of the same name elsewhere does not hide a removed one.
	mc.UpsertDescriptor(tabledesc.NewBuilder(&descpb.TableDescriptor{
		ID: 113, Name: "checks", ParentID: 100, UnexposedParentSchemaID: 101,
		Columns:            []descpb.ColumnDescriptor{{ID: 1, Name: "a", Type: types.Int}},
		PrimaryIndex:       primary,
		ViewQuery:          "SELECT shop.public.my_func() + my_func() AS a WHERE fips_ready()",
		DependsOnFunctions: []descpb.ID{106},
	}).BuildImmutable())
	mc.UpsertDescriptor(funcdesc.NewBuilder(&descpb.FunctionDescriptor{
		ID: 114, Name: "fips_ready", ParentID: 110, ParentSchemaID: 112, Lang: catpb.Function_SQL,
		ReturnType:   descpb.FunctionDescriptor_ReturnType{Type: types.Bool},
		FunctionBody: "SELECT true;",
	}).BuildImmutable())
	mc.UpsertDescriptor(funcdesc.NewBuilder(&descpb.FunctionDescriptor{
		ID: 115, Name: "calls_other", ParentID: 100, ParentSchemaID: 101, Lang: catpb.Function_SQL,
		ReturnType:   descpb.FunctionDescriptor_ReturnType{Type: types.Bool},
		FunctionBody: "SELECT fips_ready() OR other.my_func() > 0 OR my_func() > 0;",
	}).BuildImmutable())

	// User-defined types named like the spatial types.
	mc.UpsertDescriptor(schemadesc.NewBuilder(&descpb.SchemaDescriptor{
		ID: 116, Name: "my", ParentID: 100,
	}).BuildImmutable())
	mc.UpsertDescriptor(typedesc.NewBuilder(&descpb.TypeDescriptor{
		ID: 117, Name: "geometry", ParentID: 100, ParentSchemaID: 116, Kind: descpb.TypeDescriptor_ENUM,
	}).BuildImmutable())
	mc.UpsertDescriptor(typedesc.NewBuilder(&descpb.TypeDescriptor{
		ID: 119, Name: "Geometry", ParentID: 100, ParentSchemaID: 116, Kind: descpb.TypeDescriptor_ENUM,
	}).BuildImmutable())
	// Moved into public with ALTER TYPE ... SET SCHEMA.
	mc.UpsertDescriptor(typedesc.NewBuilder(&descpb.TypeDescriptor{
		ID: 122, Name: "geography", ParentID: 100, ParentSchemaID: 101, Kind: descpb.TypeDescriptor_ENUM,
	}).BuildImmutable())
	mc.UpsertDescriptor(schemadesc.NewBuilder(&descpb.SchemaDescriptor{
		ID: 120, Name: "shop", ParentID: 110,
	}).BuildImmutable())
	mc.UpsertDescriptor(typedesc.NewBuilder(&descpb.TypeDescriptor{
		ID: 121, Name: "geometry", ParentID: 110, ParentSchemaID: 120, Kind: descpb.TypeDescriptor_ENUM,
	}).BuildImmutable())
	// One view per cast, each recording the types it depends on as upstream
	// did.
	view := func(id, db, schema descpb.ID, query string, typeDeps ...descpb.ID) {
		mc.UpsertDescriptor(tabledesc.NewBuilder(&descpb.TableDescriptor{
			ID: id, Name: fmt.Sprintf("v%d", id), ParentID: db, UnexposedParentSchemaID: schema,
			Columns:        []descpb.ColumnDescriptor{{ID: 1, Name: "a", Type: types.Int}},
			PrimaryIndex:   primary,
			ViewQuery:      query,
			DependsOnTypes: typeDeps,
		}).BuildImmutable())
	}
	cast := func(typ string) string { return "SELECT 1 AS a FROM t JOIN u ON 'x'::" + typ + " IS NOT NULL" }
	view(123, 100, 101, cast("my.geometry"), 117)
	view(124, 100, 101, cast("shop.my.geometry"), 117)
	view(125, 100, 101, cast(`shop.my."Geometry"`), 119)
	// A view in another database that uses the enum of database shop.
	view(126, 110, 120, cast("my.geometry"), 117)
	// With a database of the schema's name, the name is the spatial type of
	// that database, whatever enums of that name exist elsewhere.
	view(127, 100, 101, cast("shop.geometry"))
	view(130, 100, 101, "SELECT 1 AS a FROM t JOIN u ON 'x'::world.geometry IS NOT NULL "+
		"AND 'y'::world.shop.geometry IS NOT NULL", 121)
	// IS OF records no type dependency, and v23.2 stores its types with
	// their database.
	view(128, 100, 101, "SELECT 1 IS OF (shop.my.geometry) AS a FROM shop.public.t")
	// An enum moved into public does not hide the spatial type.
	view(129, 100, 101, cast("public.geography"), 122)

	require.Equal(t, []string{
		`table shop.public.places: has a multi-region locality`,
		`table shop.public.places: column "geom" has type GEOMETRY`,
		`table shop.public.places: the computed expression of column "area" calls the removed function st_area`,
		`table shop.public.places: index "places_geom_idx" is a spatial index`,
		`table shop.public.places: index "places_id_idx" is partitioned`,
		`table shop.public.places: the predicate of index "places_id_idx" calls the removed function st_x`,
		`table shop.public.places: constraint "check_geom" calls the removed function st_isvalid`,
		`table shop.public.places: the predicate of constraint "unique_area" calls the removed function st_y`,
		`view shop.public.areas (dropped, not yet garbage collected): the query calls the removed function st_area`,
		`type shop.public.pair: element "g" has type GEOMETRY`,
		`function shop.public.plpgsql_func: is a PL/pgSQL routine`,
		`function shop.public.area: parameter "g" has type GEOMETRY`,
		`function shop.public.area: the body calls the removed function st_area`,
		`database world: is a multi-region database`,
		`type world.public.crdb_internal_region: is a multi-region enum`,
		`view shop.public.checks: the query calls the removed function fips_ready`,
		`function shop.public.calls_other: the body calls the removed functions fips_ready, my_func, other.my_func`,
		`view shop.public.v127: the query uses the removed type GEOMETRY`,
		`view shop.public.v129: the query uses the removed type GEOGRAPHY`,
		`view shop.public.v130: the query uses the removed type GEOMETRY`,
	}, catalogProblems(mc.Catalog))
}

func strPtr(s string) *string { return &s }
