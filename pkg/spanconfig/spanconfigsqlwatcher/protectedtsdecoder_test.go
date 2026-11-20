// Copyright 2021 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package spanconfigsqlwatcher_test

import (
	"context"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/jobs"
	"github.com/cockroachdb/cockroach/pkg/jobs/jobsprotectedts"
	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/protectedts/ptpb"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/protectedts/ptstorage"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/spanconfig/spanconfigsqlwatcher"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/distsql"
	"github.com/cockroachdb/cockroach/pkg/sql/isql"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/uuid"
	"github.com/stretchr/testify/require"
)

// TestProtectedTimestampDecoder verifies that we can decode a row stored in
// system.protected_ts_records.
func TestProtectedTimestampDecoder(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	ctx := context.Background()
	srv, _, kvDB := serverutils.StartServer(t, base.TestServerArgs{})
	defer srv.Stopper().Stop(ctx)
	s0 := srv.ApplicationLayer()

	ptp := s0.DistSQLServer().(*distsql.ServerImpl).ServerConfig.ProtectedTimestampProvider
	jr := s0.JobRegistry().(*jobs.Registry)
	k := s0.Codec().TablePrefix(keys.ProtectedTimestampsRecordsTableID)

	for _, testCase := range []struct {
		name   string
		target *ptpb.Target
	}{
		{
			name:   "cluster",
			target: ptpb.MakeClusterTarget(),
		},
		{
			name: "tenant",
			target: ptpb.MakeTenantsTarget([]roachpb.TenantID{roachpb.MustMakeTenantID(1),
				roachpb.MustMakeTenantID(2)}),
		},
		{
			name:   "schema-object",
			target: ptpb.MakeSchemaObjectsTarget([]descpb.ID{1, 2}),
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			ts := s0.Clock().Now()
			jobID := jr.MakeJobID()
			pts := ptstorage.WithDatabase(ptp, s0.InternalDB().(isql.DB))

			rec := jobsprotectedts.MakeRecord(
				uuid.MakeV4(), int64(jobID), ts,
				nil, /* deprecatedSpans */
				jobsprotectedts.Jobs, testCase.target,
			)
			require.NoError(t, pts.Protect(ctx, rec))

			rows, err := kvDB.Scan(ctx, k, k.PrefixEnd(), 0 /* maxRows */)
			require.NoError(t, err)
			require.Equal(t, 1, len(rows))

			last := rows[len(rows)-1]
			got, err := spanconfigsqlwatcher.TestingProtectedTimestampDecoderFn()(
				roachpb.KeyValue{
					Key:   last.Key,
					Value: *last.Value,
				},
			)
			require.NoError(t, err)
			require.Truef(t, rec.Target.Equal(got),
				"expected target=%s, got target=%s", rec.Target.String(), got.String())

			require.NoError(t, pts.Release(ctx, rec.ID.GetUUID()))
		})
	}
}
