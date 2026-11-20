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
	"strconv"

	"github.com/cockroachdb/cockroach/pkg/sql/compengine"
	"github.com/cockroachdb/cockroach/pkg/sql/comprules"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/sessiondata"
	"github.com/cockroachdb/errors"
)

// completionsNode is a shim planNode around a completionsGenerator.
// The "main" logic is in completionsGenerator, which is also
// used without a planNode by the connExecutor when running SHOW
// COMPLETIONS as an observer statement.
type completionsNode struct {
	optColumnsSlot

	n       *tree.ShowCompletions
	results compengine.Engine
}

func (n *completionsNode) startExec(params runParams) (err error) {
	override := sessiondata.InternalExecutorOverride{User: params.p.User()}
	queryIterFn := func(ctx context.Context, opName string, stmt string, args ...interface{}) (compengine.Rows, error) {
		return params.p.QueryIteratorEx(ctx, opName,
			override,
			stmt, args...)
	}
	n.results, err = newCompletionsGenerator(queryIterFn, n.n)
	return err
}

func (n *completionsNode) Next(params runParams) (bool, error) {
	return n.results.Next(params.ctx)
}

func (n *completionsNode) Values() tree.Datums {
	return n.results.Values()
}

func (n *completionsNode) Close(ctx context.Context) {
	if n.results == nil {
		return
	}
	n.results.Close(ctx)
	n.results = nil
}

func newCompletionsGenerator(
	queryIter compengine.QueryIterFn, sc *tree.ShowCompletions,
) (compengine.Engine, error) {
	offsetVal, ok := sc.Offset.AsConstantInt()
	if !ok {
		return nil, errors.Newf("invalid offset %v", sc.Offset)
	}
	offset, err := strconv.Atoi(offsetVal.String())
	if err != nil {
		return nil, err
	}
	input := sc.Statement.RawString()
	return compengine.New(queryIter, comprules.GetCompMethods(), offset, input), nil
}
