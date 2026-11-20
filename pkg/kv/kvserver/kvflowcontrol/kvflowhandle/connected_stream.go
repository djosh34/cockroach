// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package kvflowhandle

import (
	"sync/atomic"

	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/kvflowcontrol"
)

// connectedStream is a concrete implementation of the
// kvflowcontrol.ConnectedStream interface. It's used to unblock requests
// waiting for flow tokens over a stream that has since disconnected.
type connectedStream struct {
	stream       kvflowcontrol.Stream
	ch           chan struct{}
	disconnected int32
}

var _ kvflowcontrol.ConnectedStream = &connectedStream{}

func newConnectedStream(stream kvflowcontrol.Stream) *connectedStream {
	return &connectedStream{
		stream: stream,
		ch:     make(chan struct{}),
	}
}

// Stream is part of the kvflowcontrol.ConnectedStream interface.
func (b *connectedStream) Stream() kvflowcontrol.Stream {
	return b.stream
}

// Disconnected is part of the kvflowcontrol.ConnectedStream interface.
func (b *connectedStream) Disconnected() <-chan struct{} {
	return b.ch
}

// Disconnect is used to disconnect the underlying replication stream,
// unblocking all waiting requests.
func (b *connectedStream) Disconnect() {
	if atomic.CompareAndSwapInt32(&b.disconnected, 0, 1) {
		close(b.ch)
	}
}
