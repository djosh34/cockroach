// Copyright 2020 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package randgen_test

import (
	"strings"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/sql/randgen"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/randutil"
	"github.com/stretchr/testify/require"
)

func TestPostgresMutator(t *testing.T) {
	defer leaktest.AfterTest(t)()
	rng, _ := randutil.NewTestRand()
	q := `
		CREATE TABLE t (s STRING FAMILY fam1, b BYTES, FAMILY fam2 (b), PRIMARY KEY (s ASC, b DESC), INDEX (s) STORING (b), c TEXT COLLATE en_US NOT NULL);
		ALTER TABLE table1 INJECT STATISTICS 'blah';
		SET CLUSTER SETTING "sql.stats.automatic_collection.enabled" = false;
	`

	type TestCase struct {
		name string
		// original statement(s) string and mutators to apply
		original string
		mutators []randgen.Mutator
		// mutated after applying mutators
		mutated string
		changed bool
	}

	for _, testCase := range []TestCase{
		{
			name:     "postgresCreateTableMutator",
			original: q,
			mutators: []randgen.Mutator{randgen.PostgresCreateTableMutator},
			mutated:  "CREATE TABLE t (s STRING FAMILY fam1, b BYTES, FAMILY fam2 (b), PRIMARY KEY (s, b), c STRING NOT NULL);\nCREATE INDEX ON t (s) STORING (b);\nALTER TABLE table1 INJECT STATISTICS 'blah';\nSET CLUSTER SETTING \"sql.stats.automatic_collection.enabled\" = false;",
			changed:  true,
		},
		{
			name:     "postgresMutator",
			original: q,
			mutators: []randgen.Mutator{randgen.PostgresMutator},
			mutated:  `CREATE TABLE t (s TEXT, b BYTEA, PRIMARY KEY (s ASC, b DESC), INDEX (s) INCLUDE (b), c TEXT COLLATE en_US NOT NULL);`,
			changed:  true,
		},
		{
			name:     "postgresCreateTableMutator + postgresMutator",
			original: q,
			mutators: []randgen.Mutator{randgen.PostgresCreateTableMutator, randgen.PostgresMutator},
			mutated:  "CREATE TABLE t (s TEXT, b BYTEA, PRIMARY KEY (s, b), c TEXT NOT NULL);\nCREATE INDEX ON t (s) INCLUDE (b);",
			changed:  true,
		},
		{},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			actual, changed := randgen.ApplyString(rng, testCase.original, testCase.mutators...)
			require.Equal(t, testCase.changed, changed, "expected changed=%v; get %v", testCase.changed, changed)
			actual = strings.TrimSpace(actual)
			require.Equal(t, testCase.mutated, actual, "expected mutated = %v; get %v", testCase.mutated, actual)
		})
	}
}
