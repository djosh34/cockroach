// Copyright 2018 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package kvserver

import (
	"context"
	"math/rand"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/kvflowcontrol/kvflowdispatch"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/rpc"
	"github.com/cockroachdb/cockroach/pkg/rpc/nodedialer"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/util"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/netutil"
	"github.com/cockroachdb/cockroach/pkg/util/stop"
	"github.com/cockroachdb/cockroach/pkg/util/uuid"
	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/require"
)

func TestRaftTransportStartNewQueue(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)
	ctx := context.Background()

	stopper := stop.NewStopper()
	defer stopper.Stop(ctx)

	st := cluster.MakeTestingClusterSettings()
	opts := rpc.DefaultContextOptions()
	opts.Insecure = true
	opts.ToleratedOffset = 500 * time.Millisecond
	opts.Stopper = stopper
	opts.Settings = st

	rpcC := rpc.NewContext(ctx, opts)

	rpcC.StorageClusterID.Set(context.Background(), uuid.MakeV4())

	// mrs := &dummyMultiRaftServer{}

	grpcServer, err := rpc.NewServer(ctx, rpcC)
	require.NoError(t, err)
	// RegisterMultiRaftServer(grpcServer, mrs)

	var addr net.Addr

	resolver := func(roachpb.NodeID) (net.Addr, roachpb.Locality, error) {
		if addr == nil {
			return nil, roachpb.Locality{}, errors.New("no addr yet") // should not happen in this test
		}
		return addr, roachpb.Locality{}, nil
	}

	ctxWithTracer := log.MakeTestingAmbientCtxWithNewTracer()
	tp := NewRaftTransport(
		ctxWithTracer,
		cluster.MakeTestingClusterSettings(),
		ctxWithTracer.Tracer,
		nodedialer.New(rpcC, resolver),
		grpcServer,
		stopper,
		kvflowdispatch.NewDummyDispatch(),
		NoopStoresFlowControlIntegration{},
		NoopRaftTransportDisconnectListener{},
		nil, /* knobs */
	)

	ln, err := netutil.ListenAndServeGRPC(stopper, grpcServer, &util.UnresolvedAddr{NetworkField: "tcp", AddressField: "localhost:0"})
	if err != nil {
		t.Fatal(err)
	}

	addr = ln.Addr()

	defer func() {
		if ln != nil {
			_ = ln.Close()
		}
	}()

	if _, existingQueue := tp.getQueue(1, rpc.SystemClass); existingQueue {
		t.Fatal("queue already exists")
	}
	timeout := time.Duration(rand.Int63n(int64(5 * time.Millisecond)))
	log.Infof(ctx, "running test with a ctx cancellation of %s", timeout)
	ctxBoom, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		<-time.After(timeout)
		_ = ln.Close()
		ln = nil
		wg.Done()
	}()
	tp.startProcessNewQueue(ctxBoom, 1, rpc.SystemClass)

	wg.Wait()
}
