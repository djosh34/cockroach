// Copyright 2019 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package cat_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/opt/cat"
	"github.com/cockroachdb/cockroach/pkg/sql/opt/testutils/testcat"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/catconstants"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
)

func TestExpandDataSourceGlob(t *testing.T) {
	testcat := testcat.New()
	ctx := context.Background()

	exec := func(sql string) {
		if _, err := testcat.ExecuteDDL(sql); err != nil {
			t.Fatal(err)
		}
	}
	exec("CREATE TABLE a (x INT)")
	exec("CREATE TABLE b (x INT)")
	exec("CREATE TABLE c (x INT)")

	testCases := []struct {
		pattern       tree.TablePattern
		expectedNames string
		expectedIDs   string
		expectedError string
	}{
		{
			pattern:       tree.NewTableNameWithSchema("t", catconstants.PublicSchemaName, "a"),
			expectedNames: `[t.public.a]`,
			expectedIDs:   `[53]`,
		},
		{
			pattern:       tree.NewTableNameWithSchema("t", catconstants.PublicSchemaName, "z"),
			expectedError: `error: no data source matches prefix: "t.public.z"`,
		},
		{
			pattern:       &tree.AllTablesSelector{ObjectNamePrefix: tree.ObjectNamePrefix{}},
			expectedNames: `[t.public.a t.public.b t.public.c]`,
			expectedIDs:   `[53 54 55]`,
		},
		{
			pattern: &tree.AllTablesSelector{ObjectNamePrefix: tree.ObjectNamePrefix{
				SchemaName: "t", ExplicitSchema: true,
			}},
			expectedNames: `[t.public.a t.public.b t.public.c]`,
			expectedIDs:   `[53 54 55]`,
		},
		{
			pattern: &tree.AllTablesSelector{ObjectNamePrefix: tree.ObjectNamePrefix{
				SchemaName: "z", ExplicitSchema: true,
			}},
			expectedError: `error: target database or schema does not exist`,
		},
	}

	for _, tc := range testCases {
		var namesRes string
		var errRes string
		var IDsRes string
		names, IDs, err := cat.ExpandDataSourceGlob(ctx, testcat, cat.Flags{}, tc.pattern)
		if err != nil {
			errRes = fmt.Sprintf("error: %v", err)
		} else {
			var namesArr []string
			var IDsArr []descpb.ID
			for i := range names {
				namesArr = append(namesArr, names[i].FQString())
				IDsArr = append(IDsArr, IDs[i])
			}
			namesRes = fmt.Sprintf("%v", namesArr)
			IDsRes = fmt.Sprintf("%v", IDsArr)
		}
		if len(tc.expectedError) > 0 && errRes != tc.expectedError {
			t.Errorf("pattern: %v  expectedError: %s  got: %s", tc.pattern, tc.expectedError, errRes)
		}
		if len(tc.expectedNames) > 0 && namesRes != tc.expectedNames {
			t.Errorf("pattern: %v  expectedNames: %s  got: %s", tc.pattern, tc.expectedNames, namesRes)
		}
		if len(tc.expectedIDs) > 0 && IDsRes != tc.expectedIDs {
			t.Errorf("pattern: %v  expectedIDs: %s  got: %s", tc.pattern, tc.expectedIDs, IDsRes)
		}
	}
}
