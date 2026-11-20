// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

// Package indexstorageparam implements storageparam.Setter for a
// descpb.IndexDescriptor.
package indexstorageparam

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/pgwire/pgcode"
	"github.com/cockroachdb/cockroach/pkg/sql/pgwire/pgerror"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/eval"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/storageparam"
	"github.com/cockroachdb/cockroach/pkg/util/errorutil/unimplemented"
	"github.com/cockroachdb/errors"
)

// Setter observes storage parameters for indexes.
type Setter struct {
	IndexDesc *descpb.IndexDescriptor
}

var _ storageparam.Setter = (*Setter)(nil)

// Set implements the Setter interface.
func (po *Setter) Set(
	ctx context.Context,
	semaCtx *tree.SemaContext,
	evalCtx *eval.Context,
	key string,
	expr tree.Datum,
) error {
	switch key {
	case `fillfactor`:
		return storageparam.SetFillFactor(ctx, evalCtx, key, expr)
	// `bucket_count` is handled in schema changer when creating hash sharded
	// indexes.
	case `bucket_count`:
		return nil
	case `vacuum_cleanup_index_scale_factor`,
		`buffering`,
		`fastupdate`,
		`gin_pending_list_limit`,
		`pages_per_range`,
		`autosummarize`:
		return unimplemented.NewWithIssuef(43299, "storage parameter %q", key)
	}
	return pgerror.Newf(pgcode.InvalidParameterValue, "invalid storage parameter %q", key)
}

// Reset implements the StorageParameterObserver interface.
func (po *Setter) Reset(ctx context.Context, evalCtx *eval.Context, key string) error {
	return errors.AssertionFailedf("non-implemented codepath")
}

// RunPostChecks implements the Setter interface.
func (po *Setter) RunPostChecks() error {
	return nil
}
