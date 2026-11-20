// Copyright 2020 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package colexecagg

import (
	"github.com/cockroachdb/cockroach/pkg/sql/colexecop"
	"github.com/cockroachdb/cockroach/pkg/sql/colmem"
	"github.com/cockroachdb/cockroach/pkg/sql/execinfra/execagg"
	"github.com/cockroachdb/cockroach/pkg/sql/execinfrapb"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/eval"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
	"github.com/cockroachdb/cockroach/pkg/util/mon"
)

// NewAggregatorArgs encompasses all arguments necessary to instantiate either
// of the aggregators.
type NewAggregatorArgs struct {
	Allocator *colmem.Allocator
	// MemAccount should be the same as the one used by Allocator and will be
	// used by aggregatorHelper to handle DISTINCT clause.
	MemAccount     *mon.BoundAccount
	Input          colexecop.Operator
	InputTypes     []*types.T
	Spec           *execinfrapb.AggregatorSpec
	EvalCtx        *eval.Context
	Constructors   []execagg.AggregateConstructor
	ConstArguments []tree.Datums
	OutputTypes    []*types.T

	TestingKnobs struct {
		// HashTableNumBuckets if positive will override the initial number of
		// buckets to be used by the hash table (if this struct is passed to the
		// hash aggregator).
		HashTableNumBuckets uint32
	}
}

// NewHashAggregatorArgs encompasses all mandatory arguments necessary to
// instantiate the hash aggregator.
type NewHashAggregatorArgs struct {
	*NewAggregatorArgs
	HashTableAllocator       *colmem.Allocator
	OutputUnlimitedAllocator *colmem.Allocator
	MaxOutputBatchMemSize    int64
}
