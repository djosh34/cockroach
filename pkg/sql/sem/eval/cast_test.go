// Copyright 2020 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package eval

import (
	"testing"

	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/stretchr/testify/require"
)

func TestCastStringToRegClassTableName(t *testing.T) {
	defer leaktest.AfterTest(t)()

	testCases := []struct {
		in       string
		expected tree.TableName
	}{
		{"a", tree.MakeUnqualifiedTableName("a")},
		{`a"`, tree.MakeUnqualifiedTableName(`a"`)},
		{`"a""".bB."cD" `, tree.MakeTableNameWithSchema(`a"`, "bb", "cD")},
	}

	for _, tc := range testCases {
		t.Run(tc.in, func(t *testing.T) {
			out, err := castStringToRegClassTableName(tc.in)
			require.NoError(t, err)
			require.Equal(t, tc.expected, out)
		})
	}

	errorTestCases := []struct {
		in            string
		expectedError string
	}{
		{"a.b.c.d", "too many components: a.b.c.d"},
		{"", `invalid table name: `},
	}

	for _, tc := range errorTestCases {
		t.Run(tc.in, func(t *testing.T) {
			_, err := castStringToRegClassTableName(tc.in)
			require.EqualError(t, err, tc.expectedError)
		})
	}

}
