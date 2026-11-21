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
	rows, err := txn.QueryBufferedEx(ctx, "upstream-check-jobs", txn.KV(),
		sessiondata.NodeUserSessionDataOverride,
		`SELECT id, COALESCE(job_type, ''), status FROM system.jobs ORDER BY id`)
	if err != nil {
		return nil, err
	}
	var found []string
	for _, r := range rows {
		id, typ, status := int64(tree.MustBeDInt(r[0])), string(tree.MustBeDString(r[1])),
			jobs.Status(tree.MustBeDString(r[2]))
		if status.Terminal() {
			continue
		}
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
	// Views and routine bodies call user-defined functions by name.
	udfNames := make(map[string]bool)
	_ = c.ForEachDescriptor(func(d catalog.Descriptor) error {
		if fn, ok := d.(catalog.FunctionDescriptor); ok {
			udfNames[fn.GetName()] = true
		}
		return nil
	})
	var found []string
	_ = c.ForEachDescriptor(func(d catalog.Descriptor) error {
		var problems []string
		switch d := d.(type) {
		case catalog.DatabaseDescriptor:
			if d.DatabaseDesc().RegionConfig != nil {
				problems = append(problems, "is a multi-region database")
			}
		case catalog.TableDescriptor:
			problems = tableProblems(d, udfNames)
		case catalog.TypeDescriptor:
			problems = typeProblems(d)
		case catalog.FunctionDescriptor:
			problems = functionProblems(d, udfNames)
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

func tableProblems(t catalog.TableDescriptor, udfNames map[string]bool) []string {
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
			for _, p := range exprProblems(e.expr, udfNames) {
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
			for _, p := range exprProblems(idx.GetPredicate(), udfNames) {
				add("the predicate of index %q %s", idx.GetName(), p)
			}
		}
	}
	for _, ck := range t.CheckConstraints() {
		for _, p := range exprProblems(ck.GetExpr(), udfNames) {
			add("constraint %q %s", ck.GetName(), p)
		}
	}
	if ttl := t.GetRowLevelTTL(); ttl != nil && ttl.HasExpirationExpr() {
		for _, p := range exprProblems(string(ttl.ExpirationExpr), udfNames) {
			add("the TTL expiration expression %s", p)
		}
	}
	if t.IsView() {
		for _, p := range stmtProblems(t.GetViewQuery(), udfNames) {
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

func functionProblems(fn catalog.FunctionDescriptor, udfNames map[string]bool) []string {
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
	for _, p := range stmtProblems(fn.GetFunctionBody(), udfNames) {
		problems = append(problems, "the body "+p)
	}
	return problems
}

// exprProblems parses a stored expression and describes each removed
// function that it calls. An empty expression has none.
func exprProblems(s string, udfNames map[string]bool) []string {
	if s == "" {
		return nil
	}
	expr, err := parser.ParseExpr(s)
	if err != nil {
		return []string{fmt.Sprintf("cannot be parsed: %v", err)}
	}
	var calls removedCalls
	if _, err := tree.SimpleVisit(expr, calls.visitor(udfNames)); err != nil {
		return []string{err.Error()}
	}
	return calls.problems()
}

// stmtProblems is exprProblems for the statements of a view query or a
// routine body.
func stmtProblems(s string, udfNames map[string]bool) []string {
	stmts, err := parser.Parse(s)
	if err != nil {
		return []string{fmt.Sprintf("cannot be parsed: %v", err)}
	}
	var calls removedCalls
	for _, stmt := range stmts {
		if _, err := tree.SimpleStmtVisit(stmt.AST, calls.visitor(udfNames)); err != nil {
			return []string{err.Error()}
		}
	}
	return calls.problems()
}

// removedCalls collects the names of called functions that are neither
// builtins nor user-defined functions.
type removedCalls map[string]bool

func (r *removedCalls) visitor(
	udfNames map[string]bool,
) func(tree.Expr) (bool, tree.Expr, error) {
	return func(expr tree.Expr) (bool, tree.Expr, error) {
		f, ok := expr.(*tree.FuncExpr)
		if !ok {
			return true, expr, nil
		}
		// Stored expressions refer to user-defined functions by OID, which
		// needs no check.
		name, ok := f.Func.FunctionReference.(*tree.UnresolvedName)
		if !ok {
			return true, expr, nil
		}
		fn, err := name.ToRoutineName()
		if err != nil {
			return false, expr, err
		}
		def, err := tree.GetBuiltinFuncDefinition(fn, &sessiondata.DefaultSearchPath)
		if err != nil {
			return false, expr, err
		}
		if def == nil && !udfNames[fn.Object()] {
			if *r == nil {
				*r = make(removedCalls)
			}
			(*r)[fn.String()] = true
		}
		return true, expr, nil
	}
}

func (r removedCalls) problems() []string {
	if len(r) == 0 {
		return nil
	}
	names := make([]string, 0, len(r))
	for n := range r {
		names = append(names, n)
	}
	sort.Strings(names)
	if len(names) == 1 {
		return []string{"calls the removed function " + names[0]}
	}
	return []string{"calls the removed functions " + strings.Join(names, ", ")}
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
	if id := d.GetParentSchemaID(); id != descpb.InvalidID {
		schema := "public"
		if s := c.LookupDescriptor(id); s != nil {
			schema = s.GetName()
		}
		name = tree.NameString(schema) + "." + name
	}
	if id := d.GetParentID(); id != descpb.InvalidID {
		db := fmt.Sprintf("[%d]", id)
		if p := c.LookupDescriptor(id); p != nil {
			db = tree.NameString(p.GetName())
		}
		name = db + "." + name
	}
	return name
}
