// Copyright 2021 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package scexec

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/sql/schemachanger/scexec/scmutationexec"
	"github.com/cockroachdb/cockroach/pkg/sql/schemachanger/scop"
	"github.com/cockroachdb/errors"
)

// executeMutationOps will visit each operation, accumulating
// side effects into a mutationVisitorState object, and then writing out
// those side effects using the provided deps.
func executeMutationOps(
	ctx context.Context, deps Dependencies, phase scop.Phase, ops []scop.Op,
) error {
	// Execute immediate ops first.
	uvs := immediateState{}
	uv := scmutationexec.NewImmediateVisitor(&uvs, deps.Clock(), deps.Catalog())
	for _, op := range ops {
		if iop, ok := op.(scop.ImmediateMutationOp); ok {
			if err := iop.Visit(ctx, uv); err != nil {
				return errors.Wrapf(err, "%T: %v", op, op)
			}
		}
	}
	if err := uvs.exec(ctx, deps.Catalog()); err != nil {
		return err
	}
	// Exit early when in statement phase to not persist any side effects.
	if phase == scop.StatementPhase {
		return nil
	}
	// Execute deferred ops last.
	nvs := deferredState{}
	nv := scmutationexec.NewDeferredVisitor(&nvs)
	for _, op := range ops {
		if dop, ok := op.(scop.DeferredMutationOp); ok {
			if err := dop.Visit(ctx, nv); err != nil {
				return errors.Wrapf(err, "%T: %v", op, op)
			}
		}
	}
	return nvs.exec(
		ctx,
		deps.Catalog(),
		deps.TransactionalJobRegistry(),
		deps.DescriptorMetadataUpdater(ctx),
		deps.StatsRefresher(),
		deps.IndexSpanSplitter(),
	)
}
