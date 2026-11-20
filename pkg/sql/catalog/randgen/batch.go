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
)

// newDesc queues a freshly created descriptor for (later) write to KV.
// It ensures a namespace entry will also be created.
func (g *testSchemaGenerator) newDesc(ctx context.Context, desc catalog.MutableDescriptor) {
	if g.cfg.DryRun {
		return
	}
	g.output.newDescs = append(g.output.newDescs, desc)
	g.queueDescMut(ctx, desc)
}

// queueDesc queues a modified descriptor for (later) write to KV.
func (g *testSchemaGenerator) queueDescMut(ctx context.Context, desc catalog.MutableDescriptor) {
	if g.cfg.DryRun {
		return
	}
	g.output.queuedDescs = append(g.output.queuedDescs, desc)
	if len(g.output.queuedDescs) > g.cfg.BatchSize {
		g.flush(ctx)
	}
}

// flush writes pending descriptors to KV, while respecting the batch
// configuration.
func (g *testSchemaGenerator) flush(ctx context.Context) {
	if g.cfg.DryRun {
		return
	}
	if len(g.output.queuedDescs) == 0 {
		return
	}
	for _, desc := range g.output.queuedDescs {
		if err := g.ext.coll.WriteDescToBatch(ctx, g.cfg.kvTrace, desc, g.output.b); err != nil {
			panic(genError{err})
		}
	}
	for _, desc := range g.output.newDescs {
		if err := g.ext.coll.InsertNamespaceEntryToBatch(ctx, g.cfg.kvTrace, desc, g.output.b); err != nil {
			panic(genError{err})
		}
	}
	if err := g.ext.txn.Run(ctx, g.output.b); err != nil {
		panic(genError{err})
	}
	g.output.b = g.ext.txn.NewBatch()
	g.output.queuedDescs = g.output.queuedDescs[:0]
	g.output.newDescs = g.output.newDescs[:0]
}

// reset starts a new generation round.
func (g *testSchemaGenerator) reset() {
	g.cfg.GeneratedCounts.Databases = 0
	g.cfg.GeneratedCounts.Schemas = 0
	g.cfg.GeneratedCounts.Tables = 0
	g.output.queuedDescs = nil
	g.output.newDescs = nil
	g.output.b = g.ext.txn.NewBatch()
}
