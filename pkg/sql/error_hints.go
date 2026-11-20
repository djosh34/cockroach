// Copyright 2023 The Cockroach Authors.
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
	"strings"

	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/sql/deprecatedshowranges"
	"github.com/cockroachdb/cockroach/pkg/sql/pgwire/pgcode"
	"github.com/cockroachdb/cockroach/pkg/sql/pgwire/pgerror"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/eval"
	"github.com/cockroachdb/errors"
)

// addPlanningErrorHints is responsible for enhancing the provided
// error object with hints that inform the user about their options.
func addPlanningErrorHints(
	ctx context.Context, err error, stmt *Statement, st *cluster.Settings, ns eval.ClientNoticeSender,
) error {
	pgCode := pgerror.GetPGCode(err)
	switch pgCode {
	case pgcode.UndefinedColumn:
		resolvedSQL := stmt.AST.String()

		// Changes introduced in v23.1.
		extraRangeDoc := false
		switch {
		case strings.Contains(resolvedSQL, "SHOW RANGES") && !deprecatedshowranges.EnableDeprecatedBehavior(ctx, st, ns):
			errS := err.Error()

			// The following columns are not available when using SHOW
			// RANGES after the introduction of coalesced ranges.
			if strings.Contains(errS, "lease_holder") || strings.Contains(errS, "range_size") {
				err = errors.WithHint(err,
					"To list lease holder and range size details, consider SHOW RANGES WITH DETAILS.")
				extraRangeDoc = true
			}

		case strings.Contains(resolvedSQL, "crdb_internal.ranges" /* also matches ranges_no_leases */) &&
			!deprecatedshowranges.EnableDeprecatedBehavior(ctx, st, ns):
			errS := err.Error()

			// The following columns are not available when using SHOW
			// RANGES after the introduction of coalesced ranges.
			if strings.Contains(errS, "database_name") {
				err = errors.WithHint(err,
					"To list all ranges across all databases and display the database name, consider SHOW CLUSTER RANGES WITH TABLES.")
				extraRangeDoc = true
			}
			if strings.Contains(errS, "schema_name") ||
				strings.Contains(errS, "table_name") ||
				strings.Contains(errS, "table_id") {
				err = errors.WithHint(err,
					"To retrieve table/schema names/IDs associated with ranges, consider SHOW [CLUSTER] RANGES WITH TABLES.")
				extraRangeDoc = true
			}
			if strings.Contains(errS, "index_name") {
				err = errors.WithHint(err,
					"To retrieve index names associated with ranges, consider SHOW [CLUSTER] RANGES WITH INDEXES.")
				extraRangeDoc = true
			}
		}
		if extraRangeDoc {
			err = errors.WithHint(err,
				"There are more SHOW RANGES options. Refer to the online documentation or execute 'SHOW RANGES ??' for details.")
		}
	}
	return err
}
