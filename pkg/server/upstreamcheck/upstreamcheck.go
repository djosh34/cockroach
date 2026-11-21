// Copyright 2026 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

// Package upstreamcheck finds what a cluster holds that patchroaddb cut.
// Only a cluster that upstream CockroachDB v23.2 wrote can hold such
// things: jobs and schedules of cut types, spatial, multi-region and
// partitioned objects, PL/pgSQL routines, and expressions that call removed
// functions. A node runs Check before it adopts jobs, and refuses to start
// if it finds anything, so that none of it is ever run or read as a normal
// catalog object. See docs/migration.md.
package upstreamcheck

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/cockroachdb/cockroach/pkg/jobs"
	"github.com/cockroachdb/cockroach/pkg/jobs/jobspb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/catpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descs"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/nstree"
	"github.com/cockroachdb/cockroach/pkg/sql/parser"
	_ "github.com/cockroachdb/cockroach/pkg/sql/sem/builtins" // registers the builtins that names resolve to
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/sessiondata"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
	"github.com/cockroachdb/errors"
)

// cutJobTypes are the job types that patchroaddb has no code for.
var cutJobTypes = map[jobspb.Type]bool{
	jobspb.TypeBackup:                     true,
	jobspb.TypeRestore:                    true,
	jobspb.TypeChangefeed:                 true,
	jobspb.TypeReplicationStreamIngestion: true,
	jobspb.TypeReplicationStreamProducer:  true,
}

// cutScheduleExecutors are the executors of the schedule types that
// patchroaddb has no code for.
var cutScheduleExecutors = map[string]bool{
	"scheduled-backup-executor":     true,
	"scheduled-changefeed-executor": true,
}

// Check returns an error that lists everything in the cluster that uses a
// feature patchroaddb cut, or nil if there is nothing. Finished jobs of cut
// types are history and do not count.
func Check(ctx context.Context, db descs.DB) error {
	var found []string
	if err := db.DescsTxn(ctx, func(ctx context.Context, txn descs.Txn) error {
		c, err := txn.Descriptors().GetAllFromStorageUnvalidated(ctx, txn.KV())
		if err != nil {
			return err
		}
		found = catalogProblems(c)
		j, err := jobProblems(ctx, txn)
		if err != nil {
			return err
		}
		s, err := scheduleProblems(ctx, txn)
		if err != nil {
			return err
		}
		found = append(append(found, j...), s...)
		return nil
	}); err != nil {
		return errors.Wrap(err, "checking the cluster for features that patchroaddb cut")
	}
	if len(found) == 0 {
		return nil
	}
	return errors.WithHint(
		errors.Newf("this cluster uses features that patchroaddb does not have:\n  %s",
			strings.Join(found, "\n  ")),
		"Start the cluster with CockroachDB v23.2 again, drop each of these, "+
			"stop it, and start patchroaddb. docs/migration.md has the steps.")
}

func jobProblems(ctx context.Context, txn descs.Txn) ([]string, error) {
	// Finished jobs, with the statuses of jobs.Status.Terminal, do not count.
	rows, err := txn.QueryBufferedEx(ctx, "upstream-check-jobs", txn.KV(),
		sessiondata.NodeUserSessionDataOverride,
		`SELECT id, COALESCE(job_type, ''), status FROM system.jobs
		WHERE status NOT IN ($1, $2, $3, $4) ORDER BY id`,
		string(jobs.StatusSucceeded), string(jobs.StatusFailed),
		string(jobs.StatusCanceled), string(jobs.StatusRevertFailed))
	if err != nil {
		return nil, err
	}
	var found []string
	for _, r := range rows {
		id, typ, status := int64(tree.MustBeDInt(r[0])), string(tree.MustBeDString(r[1])),
			jobs.Status(tree.MustBeDString(r[2]))
		t, err := jobspb.TypeFromString(typ)
		if err != nil {
			found = append(found, fmt.Sprintf("job %d: %s job of unknown type %q", id, status, typ))
		} else if cutJobTypes[t] {
			found = append(found, fmt.Sprintf("job %d: %s %s job", id, status, typ))
		}
	}
	return found, nil
}

func scheduleProblems(ctx context.Context, txn descs.Txn) ([]string, error) {
	rows, err := txn.QueryBufferedEx(ctx, "upstream-check-schedules", txn.KV(),
		sessiondata.NodeUserSessionDataOverride,
		`SELECT schedule_id, schedule_name, executor_type FROM system.scheduled_jobs ORDER BY schedule_id`)
	if err != nil {
		return nil, err
	}
	var found []string
	for _, r := range rows {
		id, name, executor := int64(tree.MustBeDInt(r[0])), string(tree.MustBeDString(r[1])),
			string(tree.MustBeDString(r[2]))
		if cutScheduleExecutors[executor] {
			found = append(found, fmt.Sprintf("schedule %d %q: runs %s", id, name, executor))
		}
	}
	return found, nil
}

// catalogProblems lists the descriptors in c that use cut features. It reads
// them without validation, because validation rejects some of them.
func catalogProblems(c nstree.Catalog) []string {
	// User-defined types that views and routines may name like a spatial
	// type.
	var spatialNamed []catalog.Descriptor
	_ = c.ForEachDescriptor(func(d catalog.Descriptor) error {
		if d.DescriptorType() == catalog.Type && removedTypes[d.GetName()] {
			spatialNamed = append(spatialNamed, d)
		}
		return nil
	})
	udt := udtMatcher(c, spatialNamed)
	var found []string
	_ = c.ForEachDescriptor(func(d catalog.Descriptor) error {
		var problems []string
		switch d := d.(type) {
		case catalog.DatabaseDescriptor:
			if d.DatabaseDesc().RegionConfig != nil {
				problems = append(problems, "is a multi-region database")
			}
		case catalog.TableDescriptor:
			// A view records the functions it calls.
			var calls []catalog.Descriptor
			for _, id := range d.GetDependsOnFunctions() {
				if fn := c.LookupDescriptor(id); fn != nil {
					calls = append(calls, fn)
				}
			}
			problems = tableProblems(d, udfMatcher(c, calls), udt)
		case catalog.TypeDescriptor:
			problems = typeProblems(d)
		case catalog.FunctionDescriptor:
			// CockroachDB v23.2 lets no routine call a user-defined function.
			problems = functionProblems(d, noUDFs, udt)
		}
		if len(problems) == 0 {
			return nil
		}
		name := kind(d) + " " + qualifiedName(c, d)
		if d.Dropped() {
			name += " (dropped, not yet garbage collected)"
		}
		for _, p := range problems {
			found = append(found, name+": "+p)
		}
		return nil
	})
	return found
}

// isUDF reports whether a function name that an expression or statement
// calls names a user-defined function.
type isUDF func(tree.RoutineName) bool

// noUDFs is the isUDF of stored expressions, which refer to user-defined
// functions by OID instead of by name, and of routine bodies, which cannot
// call them.
func noUDFs(tree.RoutineName) bool { return false }

// udfMatcher returns an isUDF that matches the functions fns. A name that
// gives a schema or a database must match those too.
func udfMatcher(c nstree.Catalog, fns []catalog.Descriptor) isUDF {
	return func(n tree.RoutineName) bool {
		for _, fn := range fns {
			if fn.GetName() == n.Object() &&
				(!n.ExplicitSchema || schemaName(c, fn) == n.Schema()) &&
				(!n.ExplicitCatalog || dbName(c, fn) == n.Catalog()) {
				return true
			}
		}
		return false
	}
}

// isUDT reports whether a type name that an expression or statement uses
// names a user-defined type.
type isUDT func(*tree.UnresolvedObjectName) bool

// noUDTs is the isUDT of stored expressions, which refer to user-defined
// types by OID instead of by name.
func noUDTs(*tree.UnresolvedObjectName) bool { return false }

// udtMatcher returns an isUDT that matches the types typs. In CockroachDB
// v23.2 an unqualified geometry, geography or box2d is the spatial type, a
// keyword of the grammar, so only a name with a schema can match.
func udtMatcher(c nstree.Catalog, typs []catalog.Descriptor) isUDT {
	return func(n *tree.UnresolvedObjectName) bool {
		if n.NumParts == 1 {
			return false
		}
		for _, typ := range typs {
			if typ.GetName() == n.Object() && schemaName(c, typ) == n.Schema() &&
				(n.NumParts < 3 || dbName(c, typ) == n.Catalog()) {
				return true
			}
		}
		return false
	}
}

func tableProblems(t catalog.TableDescriptor, udf isUDF, udt isUDT) []string {
	var problems []string
	add := func(format string, args ...interface{}) {
		problems = append(problems, fmt.Sprintf(format, args...))
	}
	if t.TableDesc().LocalityConfig != nil {
		add("has a multi-region locality")
	}
	for _, col := range t.AllColumns() {
		if typ := col.GetType(); typ != nil && typ.ContainsRemovedSpatialType() {
			add("column %q has type %s", col.GetName(), typ.SQLString())
		}
		for _, e := range []struct{ what, expr string }{
			{"default", col.GetDefaultExpr()},
			{"ON UPDATE expression", col.GetOnUpdateExpr()},
			{"computed expression", col.GetComputeExpr()},
		} {
			for _, p := range exprProblems(e.expr) {
				add("the %s of column %q %s", e.what, col.GetName(), p)
			}
		}
	}
	for _, idx := range t.AllIndexes() {
		if cfg := idx.IndexDesc().GeoConfig; cfg.S2Geography != nil || cfg.S2Geometry != nil {
			add("index %q is a spatial index", idx.GetName())
		}
		if idx.PartitioningColumnCount() > 0 {
			add("index %q is partitioned", idx.GetName())
		}
		if idx.IsPartial() {
			for _, p := range exprProblems(idx.GetPredicate()) {
				add("the predicate of index %q %s", idx.GetName(), p)
			}
		}
	}
	for _, ck := range t.CheckConstraints() {
		for _, p := range exprProblems(ck.GetExpr()) {
			add("constraint %q %s", ck.GetName(), p)
		}
	}
	for _, uc := range t.UniqueConstraintsWithoutIndex() {
		if uc.IsPartial() {
			for _, p := range exprProblems(uc.GetPredicate()) {
				add("the predicate of constraint %q %s", uc.GetName(), p)
			}
		}
	}
	if ttl := t.GetRowLevelTTL(); ttl != nil && ttl.HasExpirationExpr() {
		for _, p := range exprProblems(string(ttl.ExpirationExpr)) {
			add("the TTL expiration expression %s", p)
		}
	}
	if t.IsView() {
		for _, p := range stmtProblems(t.GetViewQuery(), udf, udt) {
			add("the query %s", p)
		}
	}
	return problems
}

func typeProblems(t catalog.TypeDescriptor) []string {
	var problems []string
	desc := t.TypeDesc()
	if desc.Kind == descpb.TypeDescriptor_MULTIREGION_ENUM {
		problems = append(problems, "is a multi-region enum")
	}
	if desc.Composite != nil {
		for _, e := range desc.Composite.Elements {
			if e.ElementType != nil && e.ElementType.ContainsRemovedSpatialType() {
				problems = append(problems, fmt.Sprintf("element %q has type %s",
					e.ElementLabel, e.ElementType.SQLString()))
			}
		}
	}
	return problems
}

func functionProblems(fn catalog.FunctionDescriptor, udf isUDF, udt isUDT) []string {
	var problems []string
	if fn.GetLanguage() == catpb.Function_PLPGSQL {
		// The body is PL/pgSQL, which patchroaddb cannot parse.
		return append(problems, "is a PL/pgSQL routine")
	}
	for _, p := range fn.GetParams() {
		if p.Type != nil && p.Type.ContainsRemovedSpatialType() {
			problems = append(problems, fmt.Sprintf("parameter %q has type %s", p.Name, p.Type.SQLString()))
		}
	}
	if typ := fn.GetReturnType().Type; typ != nil && typ.ContainsRemovedSpatialType() {
		problems = append(problems, fmt.Sprintf("returns type %s", typ.SQLString()))
	}
	for _, p := range stmtProblems(fn.GetFunctionBody(), udf, udt) {
		problems = append(problems, "the body "+p)
	}
	return problems
}

// exprProblems parses a stored expression and describes each removed
// function that it calls. An empty expression has none.
func exprProblems(s string) []string {
	if s == "" {
		return nil
	}
	expr, err := parser.ParseExpr(s)
	if err != nil {
		return []string{fmt.Sprintf("cannot be parsed: %v", err)}
	}
	return callProblems(expr, noUDFs, noUDTs)
}

// stmtProblems is exprProblems for the statements of a view query or a
// routine body.
func stmtProblems(s string, udf isUDF, udt isUDT) []string {
	stmts, err := parser.Parse(s)
	if err != nil {
		return []string{fmt.Sprintf("cannot be parsed: %v", err)}
	}
	asts := make([]tree.Statement, len(stmts))
	for i, stmt := range stmts {
		asts[i] = stmt.AST
	}
	return callProblems(asts, udf, udt)
}

// removedTypes are the names of the removed spatial types.
var removedTypes = map[string]bool{"geometry": true, "geography": true, "box2d": true}

// callProblems describes the functions that the syntax tree node calls and
// that are neither builtins nor user-defined functions, and the removed types
// that it casts to.
func callProblems(node interface{}, udf isUDF, udt isUDT) []string {
	calls, typeRefs := walkSyntax(node)
	removed := make(map[string]bool)
	for _, f := range calls {
		// A reference by OID is to a user-defined function.
		name, ok := f.Func.FunctionReference.(*tree.UnresolvedName)
		if !ok {
			continue
		}
		fn, err := name.ToRoutineName()
		if err != nil {
			return []string{fmt.Sprintf("cannot be checked: %v", err)}
		}
		def, err := tree.GetBuiltinFuncDefinition(fn, &sessiondata.DefaultSearchPath)
		if err != nil {
			return []string{fmt.Sprintf("cannot be checked: %v", err)}
		}
		if def == nil && !udf(fn) {
			removed[fn.String()] = true
		}
	}
	var problems []string
	if len(removed) > 0 {
		problems = append(problems, describe("calls the removed function", removed))
	}
	// A spatial type name is a removed type unless it names a user-defined
	// type.
	removed = make(map[string]bool)
	for _, ref := range typeRefs {
		for {
			a, ok := ref.(*tree.ArrayTypeReference)
			if !ok {
				break
			}
			ref = a.ElementType
		}
		switch t := ref.(type) {
		case *tree.UnresolvedObjectName:
			if name := strings.ToLower(t.Object()); removedTypes[name] && !udt(t) {
				removed[strings.ToUpper(name)] = true
			}
		case *types.T:
			if t.ContainsRemovedSpatialType() {
				removed[t.SQLString()] = true
			}
		}
	}
	if len(removed) > 0 {
		problems = append(problems, describe("uses the removed type", removed))
	}
	return problems
}

// describe returns what followed by the sorted names, with what in the
// plural for more than one name.
func describe(what string, names map[string]bool) string {
	sorted := make([]string, 0, len(names))
	for n := range names {
		sorted = append(sorted, n)
	}
	sort.Strings(sorted)
	if len(sorted) > 1 {
		what += "s"
	}
	return what + " " + strings.Join(sorted, ", ")
}

var (
	funcExprType     = reflect.TypeOf((*tree.FuncExpr)(nil))
	castExprType     = reflect.TypeOf((*tree.CastExpr)(nil))
	annotateExprType = reflect.TypeOf((*tree.AnnotateTypeExpr)(nil))
	isOfTypeExprType = reflect.TypeOf((*tree.IsOfTypeExpr)(nil))
)

// walkSyntax returns every function call in a syntax tree, and the types of
// every cast, type annotation and IS OF test. The tree package's visitors
// skip some expressions, such as join conditions and window definitions, so
// this follows every exported field instead.
func walkSyntax(
	node interface{},
) (calls []*tree.FuncExpr, typeRefs []tree.ResolvableTypeReference) {
	var walk func(v reflect.Value)
	walk = func(v reflect.Value) {
		switch v.Kind() {
		case reflect.Pointer, reflect.Interface:
			if v.IsNil() {
				return
			}
			switch v.Type() {
			case funcExprType:
				calls = append(calls, v.Interface().(*tree.FuncExpr))
			case castExprType:
				typeRefs = append(typeRefs, v.Interface().(*tree.CastExpr).Type)
			case annotateExprType:
				typeRefs = append(typeRefs, v.Interface().(*tree.AnnotateTypeExpr).Type)
			case isOfTypeExprType:
				typeRefs = append(typeRefs, v.Interface().(*tree.IsOfTypeExpr).Types...)
			}
			walk(v.Elem())
		case reflect.Struct:
			for i := 0; i < v.NumField(); i++ {
				if v.Type().Field(i).IsExported() {
					walk(v.Field(i))
				}
			}
		case reflect.Slice, reflect.Array:
			for i := 0; i < v.Len(); i++ {
				walk(v.Index(i))
			}
		}
	}
	walk(reflect.ValueOf(node))
	return calls, typeRefs
}

// kind names the kind of object that d describes.
func kind(d catalog.Descriptor) string {
	if t, ok := d.(catalog.TableDescriptor); ok {
		switch {
		case t.IsView():
			return "view"
		case t.IsSequence():
			return "sequence"
		}
		return "table"
	}
	return string(d.DescriptorType())
}

// qualifiedName returns db.schema.name for a descriptor in a database, and
// its name otherwise.
func qualifiedName(c nstree.Catalog, d catalog.Descriptor) string {
	name := tree.NameString(d.GetName())
	if d.GetParentSchemaID() != descpb.InvalidID {
		name = tree.NameString(schemaName(c, d)) + "." + name
	}
	if d.GetParentID() != descpb.InvalidID {
		name = tree.NameString(dbName(c, d)) + "." + name
	}
	return name
}

// schemaName returns the name of d's schema.
func schemaName(c nstree.Catalog, d catalog.Descriptor) string {
	if s := c.LookupDescriptor(d.GetParentSchemaID()); s != nil {
		return s.GetName()
	}
	return "public"
}

// dbName returns the name of d's database.
func dbName(c nstree.Catalog, d catalog.Descriptor) string {
	if db := c.LookupDescriptor(d.GetParentID()); db != nil {
		return db.GetName()
	}
	return fmt.Sprintf("[%d]", d.GetParentID())
}
