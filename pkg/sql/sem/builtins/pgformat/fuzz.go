// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

//go:build gofuzz
// +build gofuzz

package pgformat

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/sql/sem/eval"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
)

// FuzzFormat passes the input to pgformat.Format()
// as both the format string and format arguments.
func FuzzFormat(input []byte) int {
	evalCtx := eval.MakeTestingEvalContext(nil)
	str := string(input)
	args := make(tree.Datums, 16)
	for i := range args {
		args[i] = tree.NewDString(string(input))
	}
	_, err := Format(context.Background(), &evalCtx, str, args...)

	if err == nil {
		return 0
	}
	return 1
}
