// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package keyvisualizer

import "context"

// SpanStatsConsumer runs in the tenant key visualizer job.
// It is the tenant's API to control statistic collection in KV.
type SpanStatsConsumer interface {

	// UpdateBoundaries decides the boundaries that statistics should be collected for, and communicates them to KV.
	// Updates are applied on every node after the following events occur:
	// 1) The update payload propagates to collectors.
	// 2) The collector rolls over its current sample and the boundary update time is in the past.
	// The timestamp helps us make sure that rangefeed propagation delays don't result in collectors
	// installing different boundaries.
	UpdateBoundaries(context.Context) error

	// GetSamples retrieves the latest samples from KV, and then downsamples and persists them.
	GetSamples(context.Context) error

	// DeleteExpiredSamples deletes historical samples older than 2 weeks.
	DeleteExpiredSamples(context.Context) error
}
