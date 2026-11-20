// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package bootstrap_test

import (
	"context"
	"math/rand"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/config/zonepb"
	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/bootstrap"
	"github.com/cockroachdb/cockroach/pkg/testutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/testcluster"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/randutil"
	"github.com/stretchr/testify/require"
)

// TestInitialSplitPoints ensures that any split points that are generated
// during cluster creation are valid split points. The test runs for both the
// system tenant and secondary tenants; the ID of the latter is generated at
// random -- see https://github.com/cockroachdb/cockroach/issues/104928 for why
// this is interesting.
func TestInitialSplitPoints(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	testutils.RunTrueAndFalse(t, "tenant", func(t *testing.T, isSystem bool) {
		codec := keys.SystemSQLCodec
		if !isSystem {
			seed := randutil.NewPseudoSeed()
			t.Logf("seed is %d", seed)
			rng := rand.New(rand.NewSource(seed))
			tenID := rng.Uint64()
			t.Logf("tenant ID is %d", tenID)
			codec = keys.MakeSQLCodec(roachpb.MustMakeTenantID(tenID))
		}
		opts := bootstrap.InitialValuesOpts{
			DefaultZoneConfig:       zonepb.DefaultZoneConfigRef(),
			DefaultSystemZoneConfig: zonepb.DefaultZoneConfigRef(),
			OverrideKey:             0, // most recent
			Codec:                   codec,
		}

		ctx := context.Background()
		tc := testcluster.StartTestCluster(t, 1, base.TestClusterArgs{
			ReplicationMode: base.ReplicationManual,
		})
		defer tc.Stopper().Stop(ctx)

		_, splits, err := opts.GenerateInitialValues()
		require.NoError(t, err)

		for _, split := range splits {
			_, _, err = tc.SplitRange(roachpb.Key(split))
			require.NoError(t, err)
		}
	})
}
