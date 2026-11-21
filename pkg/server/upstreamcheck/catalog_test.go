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
	"testing"

	"github.com/cockroachdb/cockroach/pkg/sql/catalog/catpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/dbdesc"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/funcdesc"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/nstree"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/schemadesc"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/tabledesc"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/typedesc"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/stretchr/testify/require"
)

func TestExprProblems(t *testing.T) {
	defer leaktest.AfterTest(t)()
	udfs := map[string]bool{"my_func": true}
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
		{"my_func(a) + public.my_func(b)", nil},
		{"st_area(geom) > 1", []string{"calls the removed function st_area"}},
		{"st_x(a) + st_area(a) + st_x(b)", []string{"calls the removed functions st_area, st_x"}},
		{"crdb_internal.fips_ready()", []string{"calls the removed function crdb_internal.fips_ready"}},
		{"decrypt(a, b, 'aes')", []string{"calls the removed function decrypt"}},
		{"(", []string{`cannot be parsed: at or near "EOF": syntax error`}},
	} {
		t.Run(tc.expr, func(t *testing.T) {
			require.Equal(t, tc.want, exprProblems(tc.expr, udfs))
		})
	}
}

func TestStmtProblems(t *testing.T) {
	defer leaktest.AfterTest(t)()
	for _, tc := range []struct {
		stmts string
		want  []string
	}{
		{"SELECT a, count(*) FROM t GROUP BY a", nil},
		{"SELECT st_x(g) FROM t", []string{"calls the removed function st_x"}},
		{"SELECT a FROM (SELECT st_x(g) AS a FROM t) WHERE a IN (SELECT st_y(g) FROM u)",
			[]string{"calls the removed functions st_x, st_y"}},
		{"SELECT 1; SELECT st_x(g) FROM t;", []string{"calls the removed function st_x"}},
	} {
		t.Run(tc.stmts, func(t *testing.T) {
			require.Equal(t, tc.want, stmtProblems(tc.stmts, nil))
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
		Checks:         []*descpb.TableDescriptor_CheckConstraint{{Name: "check_geom", Expr: "st_isvalid(geom)"}},
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
		ID: 107, Name: "uses_my_func", ParentID: 100, ParentSchemaID: 101, Lang: catpb.Function_SQL,
		ReturnType:   descpb.FunctionDescriptor_ReturnType{Type: types.Int},
		FunctionBody: "SELECT my_func() + length('x');",
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

	require.Equal(t, []string{
		`table shop.public.places: has a multi-region locality`,
		`table shop.public.places: column "geom" has type GEOMETRY`,
		`table shop.public.places: the computed expression of column "area" calls the removed function st_area`,
		`table shop.public.places: index "places_geom_idx" is a spatial index`,
		`table shop.public.places: index "places_id_idx" is partitioned`,
		`table shop.public.places: the predicate of index "places_id_idx" calls the removed function st_x`,
		`table shop.public.places: constraint "check_geom" calls the removed function st_isvalid`,
		`view shop.public.areas (dropped, not yet garbage collected): the query calls the removed function st_area`,
		`type shop.public.pair: element "g" has type GEOMETRY`,
		`function shop.public.plpgsql_func: is a PL/pgSQL routine`,
		`function shop.public.area: parameter "g" has type GEOMETRY`,
		`function shop.public.area: the body calls the removed function st_area`,
		`database world: is a multi-region database`,
		`type world.public.crdb_internal_region: is a multi-region enum`,
	}, catalogProblems(mc.Catalog))
}

func strPtr(s string) *string { return &s }
