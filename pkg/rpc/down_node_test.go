// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.
//

package rpc

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/testutils"
	"github.com/cockroachdb/cockroach/pkg/util"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/netutil"
	"github.com/cockroachdb/cockroach/pkg/util/stop"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
	"github.com/cockroachdb/cockroach/pkg/util/uuid"
	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/require"
)

func TestConnectingToDownNode(t *testing.T) {
	defer leaktest.AfterTest(t)()

	{
		defer func(prev time.Duration) {
			base.DialTimeout = prev
		}(base.DialTimeout)
		base.DialTimeout = time.Millisecond
	}

	testutils.RunTrueAndFalse(t, "refused", func(t *testing.T, refused bool) {
		ctx := context.Background()

		rpcCtx := newTestContext(uuid.FastMakeV4(), &timeutil.DefaultTimeSource{}, time.Second, stop.NewStopper())
		defer rpcCtx.Stopper.Stop(ctx)
		rpcCtx.NodeID.Set(context.Background(), 1)

		ln, err := net.Listen("tcp", util.TestAddr.AddressField)
		require.NoError(t, err)
		if refused {
			require.NoError(t, ln.Close())
		} else {
			// Simulate a network black hole by opening a listener but never
			// Accept()ing from it. Not quite the real thing, but close enough.
			defer func() { require.NoError(t, ln.Close()) }()
		}

		var dur time.Duration
		const n = 100
		for i := 0; i < n; i++ {
			tBegin := timeutil.Now()
			_, err = rpcCtx.GRPCDialNode(ln.Addr().String(), 1, roachpb.Locality{}, DefaultClass).
				Connect(ctx)
			require.True(t, errors.HasType(err, (*netutil.InitialHeartbeatFailedError)(nil)), "%+v", err)
			dur += timeutil.Since(tBegin)
		}
		avg := dur / n
		t.Logf("avg duration/attempt: %s", avg)
		require.Less(t, avg, 100*time.Millisecond)
	})
}
