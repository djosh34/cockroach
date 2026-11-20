// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package bench

import (
	"context"
	"fmt"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/sqlutils"
	"github.com/cockroachdb/cockroach/pkg/util/log"
)

func runUniqBench(
	b *testing.B,
	setup func(b *testing.B, r *sqlutils.SQLRunner),
	run func(b *testing.B, r *sqlutils.SQLRunner),
) {
	configs := []struct {
		name           string
		insertFastPath bool
	}{
		{name: "NoFastPath", insertFastPath: false},
		{name: "FastPath", insertFastPath: true},
	}

	for _, cfg := range configs {
		b.Run(cfg.name, func(b *testing.B) {
			s, db, _ := serverutils.StartServer(b, base.TestServerArgs{})
			defer s.Stopper().Stop(context.Background())
			r := sqlutils.MakeSQLRunner(db)
			// Don't let auto stats interfere with the test. Stock stats are
			// sufficient to get the right plans (i.e. lookup join).
			r.Exec(b, "SET CLUSTER SETTING sql.stats.automatic_collection.enabled = false")
			r.Exec(b, "SET experimental_enable_unique_without_index_constraints = true")
			r.Exec(b, fmt.Sprintf("SET enable_insert_fast_path = %v", cfg.insertFastPath))
			setup(b, r)
			b.ResetTimer()
			run(b, r)
		})
	}
}

func BenchmarkUniqInsert(b *testing.B) {
	defer log.Scope(b).Close(b)

	const numRows = 1000
	setup := func(b *testing.B, r *sqlutils.SQLRunner) {
		r.Exec(b, "CREATE TABLE t1 (k int, p int, INDEX (k), UNIQUE WITHOUT INDEX (k))")
	}

	b.Run("SingleRow", func(b *testing.B) {
		runUniqBench(b, setup, func(b *testing.B, r *sqlutils.SQLRunner) {
			for i := 0; i < b.N; i++ {
				r.Exec(b, fmt.Sprintf("INSERT INTO t1 VALUES (%d, %d)", i, i%numRows))
			}
		})
	})
}
