// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package rangefeedcache_test

import (
	"context"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/kv/kvclient/rangefeed"
	"github.com/cockroachdb/cockroach/pkg/kv/kvclient/rangefeed/rangefeedbuffer"
	"github.com/cockroachdb/cockroach/pkg/kv/kvclient/rangefeed/rangefeedcache"
	"github.com/cockroachdb/cockroach/pkg/kv/kvpb"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/util/grpcutil"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/stretchr/testify/require"
)

func TestWatchAuthErr(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	ctx := context.Background()
	host := serverutils.StartServerOnly(t, base.TestServerArgs{
		DefaultTestTenant: base.TestControlsTenantsExplicitly,
	})
	defer host.Stopper().Stop(ctx)

	tenant, _ := serverutils.StartTenant(t, host, base.TestTenantArgs{
		TenantID: serverutils.TestTenantID(),
	})

	hostScratchRange, err := host.ScratchRange()
	require.NoError(t, err)
	hostScratchSpan := roachpb.Span{
		Key:    hostScratchRange,
		EndKey: hostScratchRange.PrefixEnd(),
	}

	w := rangefeedcache.NewWatcher(
		"test",
		tenant.Clock(),
		tenant.RangeFeedFactory().(*rangefeed.Factory),
		1024,
		[]roachpb.Span{hostScratchSpan},
		false, /* withPrevValue */
		true,  /* withRowTSInInitialScan */
		func(ctx context.Context, kv *kvpb.RangeFeedValue) rangefeedbuffer.Event {
			t.Fatalf("rangefeed should fail before producing results")
			return nil
		},
		func(ctx context.Context, update rangefeedcache.Update) {
			t.Fatalf("rangefeed should fail before producing results")
		},
		&rangefeedcache.TestingKnobs{})

	err = w.Run(ctx)
	require.True(t, grpcutil.IsAuthError(err), "expected %v to be an auth error", err)
}
