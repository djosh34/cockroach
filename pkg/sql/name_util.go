// Copyright 2021 The Cockroach Authors.
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

	"github.com/cockroachdb/cockroach/pkg/kv"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
)

func (p *planner) dropNamespaceEntry(
	ctx context.Context, b *kv.Batch, oldNameKey catalog.NameKey,
) error {
	return p.Descriptors().DeleteNamespaceEntryToBatch(
		ctx, p.ExtendedEvalContext().Tracing.KVTracingEnabled(), oldNameKey, b,
	)
}

func (p *planner) renameNamespaceEntry(
	ctx context.Context, b *kv.Batch, oldNameKey catalog.NameKey, desc catalog.MutableDescriptor,
) error {
	if err := p.dropNamespaceEntry(ctx, b, oldNameKey); err != nil {
		return err
	}
	return p.Descriptors().InsertNamespaceEntryToBatch(
		ctx, p.ExtendedEvalContext().Tracing.KVTracingEnabled(), desc, b,
	)
}
