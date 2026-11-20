// Copyright 2021 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package lease

import (
	"context"
	"fmt"
	"strings"

	"github.com/cockroachdb/cockroach/pkg/clusterversion"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/sql/enum"
	"github.com/cockroachdb/cockroach/pkg/sql/isql"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/sessiondata"
	"github.com/cockroachdb/cockroach/pkg/util/hlc"
	"github.com/cockroachdb/errors"
)

// getSingleRegionClause returns an extra clause that limits scans on the
// system.leases table to the single region all leases are written to.
func getSingleRegionClause(
	ctx context.Context, st *cluster.Settings,
) (query string, queryParam []byte) {
	query, queryParam = "", nil
	if st.Version.IsActive(ctx, clusterversion.V23_1_SystemRbrSingleWrite) {
		query = " AND crdb_region=$2"
		queryParam = enum.One
	}
	return query, queryParam
}

// CountLeases returns the number of unexpired leases for a number of descriptors
// each at a particular version at a particular time.
func CountLeases(
	ctx context.Context,
	executor isql.Executor,
	st *cluster.Settings,
	versions []IDVersion,
	at hlc.Timestamp,
) (int, error) {
	whereClauses := strings.Builder{}
	for i, t := range versions {
		if i != 0 {
			whereClauses.WriteString(" OR ")
		}
		whereClauses.WriteString(fmt.Sprintf(`("descID" = %d AND version = %d AND expiration > $1)`,
			t.ID, t.Version))
	}

	singleRegionClause, singleRegionParam := getSingleRegionClause(ctx, st)
	stmt := fmt.Sprintf(`SELECT count(1) FROM system.public.lease AS OF SYSTEM TIME '%s' WHERE (%s) %s `,
		at.AsOfSystemTime(),
		whereClauses.String(),
		singleRegionClause)

	values, err := executor.QueryRowEx(
		ctx, "count-leases", nil, /* txn */
		sessiondata.RootUserSessionDataOverride,
		stmt, at.GoTime(), singleRegionParam,
	)
	if err != nil {
		return 0, err
	}
	if values == nil {
		return 0, errors.New("failed to count leases")
	}
	count := int(tree.MustBeDInt(values[0]))
	return count, nil
}
