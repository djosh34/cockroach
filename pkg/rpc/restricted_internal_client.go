// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package rpc

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/kv/kvpb"
	"google.golang.org/grpc"
)

// RestrictedInternalClient represents the part of the kvpb.InternalClient
// interface used by the DistSender. Besides the auto-generated gRPC client,
// this interface is also implemented by rpc.internalClientAdapter which
// bypasses gRPC to call into the local Node.
//
// For a more contextualized explanation, see the comment that decorates
// (*rpc.Context).loopbackDialFn.
type RestrictedInternalClient interface {
	Batch(ctx context.Context, in *kvpb.BatchRequest, opts ...grpc.CallOption) (*kvpb.BatchResponse, error)
	RangeFeed(ctx context.Context, in *kvpb.RangeFeedRequest, opts ...grpc.CallOption) (kvpb.Internal_RangeFeedClient, error)
	MuxRangeFeed(ctx context.Context, opts ...grpc.CallOption) (kvpb.Internal_MuxRangeFeedClient, error)
}
