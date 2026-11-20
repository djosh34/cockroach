// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

// This module holds functions related to building insert fast path
// foreign key and uniqueness checks and deal mostly with structures
// defined in the memo package.

package memo

import (
	"github.com/cockroachdb/cockroach/pkg/sql/mutations"
	"github.com/cockroachdb/cockroach/pkg/sql/opt"
)

// ValuesLegalForInsertFastPath tests if `values` is a Values expression that
// has no subqueries or UDFs and has less rows than the max number of entries in
// a KV batch for a mutation operation.
func ValuesLegalForInsertFastPath(values *ValuesExpr) bool {
	//  - The input is Values with at most mutations.MaxBatchSize, and there are no
	//    subqueries;
	//    (note that mutations.MaxBatchSize() is a quantity of keys in the batch
	//     that we send, not a number of rows. We use this as a guideline only,
	//     and there is no guarantee that we won't produce a bigger batch.)
	if values.ChildCount() > mutations.MaxBatchSize(false /* forceProductionMaxBatchSize */) ||
		values.Relational().HasSubquery ||
		values.Relational().HasUDF {
		return false
	}
	return true
}

// CanAutoCommit determines if it is safe to auto commit the mutation contained
// in the expression.
//
// Mutations can commit the transaction as part of the same KV request,
// potentially taking advantage of the 1PC optimization. This is not ok to do in
// general; a sufficient set of conditions is:
//  1. There is a single mutation in the query.
//  2. The mutation is the root operator, or it is directly under a Project
//     with no side-effecting expressions. An example of why we can't allow
//     side-effecting expressions: if the projection encounters a
//     division-by-zero error, the mutation shouldn't have been committed.
//
// An extra condition relates to how the FK checks are run. If they run before
// the mutation (via the insert fast path), auto commit is possible. If they run
// after the mutation (the general path), auto commit is not possible. It is up
// to the builder logic for each mutation to handle this.
//
// Note that there are other necessary conditions related to execution
// (specifically, that the transaction is implicit); it is up to the exec
// factory to take that into account as well.
func CanAutoCommit(rel RelExpr) bool {
	if !rel.Relational().CanMutate {
		// No mutations in the expression.
		return false
	}

	switch rel.Op() {
	case opt.InsertOp, opt.UpsertOp, opt.UpdateOp, opt.DeleteOp:
		// Check that there aren't any more mutations in the input.
		// TODO(radu): this can go away when all mutations are under top-level
		// With ops.
		return !rel.Child(0).(RelExpr).Relational().CanMutate

	case opt.ProjectOp:
		// Allow Project on top, as long as the expressions are not side-effecting.
		proj := rel.(*ProjectExpr)
		for i := 0; i < len(proj.Projections); i++ {
			if !proj.Projections[i].ScalarProps().VolatilitySet.IsLeakproof() {
				return false
			}
		}
		return CanAutoCommit(proj.Input)

	case opt.DistributeOp:
		// Distribute is currently a no-op, so check whether the input can
		// auto-commit.
		return CanAutoCommit(rel.(*DistributeExpr).Input)

	default:
		return false
	}
}
