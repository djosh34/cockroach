// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package tree_test

import (
	"testing"

	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/stretchr/testify/require"
)

func TestConflictingFunctionOptions(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	testCases := []struct {
		testName    string
		options     tree.RoutineOptions
		expectedErr string
	}{
		{
			testName: "no conflict",
			options: tree.RoutineOptions{
				tree.RoutineVolatile, tree.RoutineLeakproof(true), tree.RoutineCalledOnNullInput, tree.RoutineLangSQL, tree.RoutineBodyStr("hi"),
			},
			expectedErr: "",
		},
		{
			testName: "volatility conflict",
			options: tree.RoutineOptions{
				tree.RoutineVolatile, tree.RoutineStable,
			},
			expectedErr: "STABLE: conflicting or redundant options",
		},
		{
			testName: "null input behavior conflict 1",
			options: tree.RoutineOptions{
				tree.RoutineCalledOnNullInput, tree.RoutineReturnsNullOnNullInput,
			},
			expectedErr: "RETURNS NULL ON NULL INPUT: conflicting or redundant options",
		},
		{
			testName: "null input behavior conflict 2",
			options: tree.RoutineOptions{
				tree.RoutineCalledOnNullInput, tree.RoutineStrict,
			},
			expectedErr: "STRICT: conflicting or redundant options",
		},
		{
			testName: "leakproof conflict",
			options: tree.RoutineOptions{
				tree.RoutineLeakproof(true), tree.RoutineLeakproof(false),
			},
			expectedErr: "NOT LEAKPROOF: conflicting or redundant options",
		},
		{
			testName: "language conflict",
			options: tree.RoutineOptions{
				tree.RoutineLangSQL, tree.RoutineLangSQL,
			},
			expectedErr: "LANGUAGE SQL: conflicting or redundant options",
		},
		{
			testName: "function body conflict",
			options: tree.RoutineOptions{
				tree.RoutineBodyStr("queries"), tree.RoutineBodyStr("others"),
			},
			expectedErr: "AS $$others$$: conflicting or redundant options",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			err := tree.ValidateRoutineOptions(tc.options)
			if tc.expectedErr == "" {
				require.NoError(t, err)
				return
			}
			require.Equal(t, tc.expectedErr, err.Error())
		})
	}
}
