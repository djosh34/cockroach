// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package sql

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/sql/catalog/catalogkeys"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
)

func (p *planner) updateComment(
	ctx context.Context, objID descpb.ID, subID uint32, cmtType catalogkeys.CommentType, cmt string,
) error {
	b := p.Txn().NewBatch()
	if err := p.descCollection.WriteCommentToBatch(
		ctx,
		p.ExtendedEvalContext().Tracing.KVTracingEnabled(),
		b,
		catalogkeys.MakeCommentKey(uint32(objID), subID, cmtType),
		cmt,
	); err != nil {
		return err
	}
	return p.Txn().Run(ctx, b)
}

func (p *planner) deleteComment(
	ctx context.Context, objID descpb.ID, subID uint32, cmtType catalogkeys.CommentType,
) error {
	b := p.Txn().NewBatch()
	if err := p.descCollection.DeleteCommentInBatch(
		ctx,
		p.ExtendedEvalContext().Tracing.KVTracingEnabled(),
		b,
		catalogkeys.MakeCommentKey(uint32(objID), subID, cmtType),
	); err != nil {
		return err
	}
	return p.Txn().Run(ctx, b)
}
