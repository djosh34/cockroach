// Copyright 2019 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package flowinfra

import (
	"context"
	"io"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/rpc"
	"github.com/cockroachdb/cockroach/pkg/sql/execinfra"
	"github.com/cockroachdb/cockroach/pkg/sql/execinfrapb"
	"github.com/cockroachdb/cockroach/pkg/util/hlc"
	"github.com/cockroachdb/cockroach/pkg/util/stop"
)

// createDummyStream creates the server and client side of a FlowStream stream.
// This can be use by tests to pretend that they have received a FlowStream RPC.
// The stream can be used to send messages (ConsumerSignal's) on it (within a
// gRPC window limit since nobody's reading from the stream), for example
// Handshake messages.
//
// We do this by creating a mock server, dialing into it and capturing the
// server stream. The server-side RPC call will be blocked until the caller
// calls the returned cleanup function. The cleanup function also "drains" the
// client-side stream.
func createDummyStream(
	t *testing.T,
) (
	serverStream execinfrapb.DistSQL_FlowStreamServer,
	clientStream execinfrapb.DistSQL_FlowStreamClient,
	cleanup func(),
) {
	stopper := stop.NewStopper()
	ctx := context.Background()
	clock := hlc.NewClockForTesting(nil)
	storageClusterID, mockServer, addr, err := execinfrapb.StartMockDistSQLServer(ctx, clock, stopper, execinfra.StaticSQLInstanceID)
	if err != nil {
		t.Fatal(err)
	}

	rpcContext := rpc.NewInsecureTestingContextWithClusterID(ctx, clock, stopper, storageClusterID)
	conn, err := rpcContext.GRPCDialNode(addr.String(), roachpb.NodeID(execinfra.StaticSQLInstanceID),
		roachpb.Locality{}, rpc.DefaultClass).Connect(ctx)
	if err != nil {
		t.Fatal(err)
	}
	client := execinfrapb.NewDistSQLClient(conn)
	clientStream, err = client.FlowStream(ctx)
	if err != nil {
		t.Fatal(err)
	}
	streamNotification := <-mockServer.InboundStreams
	serverStream = streamNotification.Stream
	cleanup = func() {
		close(streamNotification.Donec)
		// After the RPC is unblocked, we have to drain the client side in order
		// to simulate what happens in production (where the watchdog goroutine
		// of the outbox does this).
		for {
			_, err := clientStream.Recv()
			if err != nil {
				if err == io.EOF {
					break
				}
				t.Fatal(err)
			}
		}
		stopper.Stop(ctx)
	}
	return serverStream, clientStream, cleanup
}
