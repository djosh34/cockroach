// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package spanstatskvaccessor

import (
	"context"
	"time"

	"github.com/cockroachdb/cockroach/pkg/keyvisualizer/keyvispb"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
)

// SpanStatsKVAccessor provides an API for co-located system tenant to interact with KV.
type SpanStatsKVAccessor struct {
	server keyvispb.KeyVisualizerServer
}

// New constructs a new SpanStatsKVAccessor.
func New(server keyvispb.KeyVisualizerServer) *SpanStatsKVAccessor {
	return &SpanStatsKVAccessor{server: server}
}

// UpdateBoundaries invokes the keyvispb.UpdateBoundaries rpc.
func (s *SpanStatsKVAccessor) UpdateBoundaries(
	ctx context.Context, boundaries []roachpb.Span, timestamp time.Time,
) (*keyvispb.UpdateBoundariesResponse, error) {

	return s.server.UpdateBoundaries(ctx, &keyvispb.UpdateBoundariesRequest{
		Boundaries: boundaries,
		Time:       timestamp,
	})

}

// GetSamples invokes the keyvispb.GetSamples rpc.
func (s *SpanStatsKVAccessor) GetSamples(
	ctx context.Context, timestamp time.Time,
) (*keyvispb.GetSamplesResponse, error) {

	return s.server.GetSamples(ctx, &keyvispb.GetSamplesRequest{
		NodeID:             0, // 0 indicates the server should be a fan-out.
		CollectedOnOrAfter: timestamp,
	})

}
