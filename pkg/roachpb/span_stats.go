// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package roachpb

import (
	"time"

	"github.com/cockroachdb/cockroach/pkg/settings"
)

// Put span statistics cluster settings here to avoid import cycle.

const DefaultSpanStatsSpanLimit = 1000

// SpanStatsBatchLimit registers the maximum number of spans allowed in a
// span stats request payload.
var SpanStatsBatchLimit = settings.RegisterIntSetting(
	settings.ApplicationLevel,
	"server.span_stats.span_batch_limit",
	"the maximum number of spans allowed in a request payload for span statistics",
	DefaultSpanStatsSpanLimit,
	settings.PositiveInt,
)

var SpanStatsNodeTimeout = settings.RegisterDurationSetting(
	settings.ApplicationLevel,
	"server.span_stats.node.timeout",
	"the duration allowed for a single node to return span stats data before"+
		" the request is cancelled; if set to 0, there is no timeout",
	time.Minute,
	settings.NonNegativeDuration,
)

const defaultRangeStatsBatchLimit = 100

// RangeStatsBatchLimit registers the maximum number of ranges to be batched
// when fetching range stats for a span.
var RangeStatsBatchLimit = settings.RegisterIntSetting(
	settings.ApplicationLevel,
	"server.span_stats.range_batch_limit",
	"the maximum batch size when fetching ranges statistics for a span",
	defaultRangeStatsBatchLimit,
	settings.PositiveInt,
)

// RangeDescPageSize controls the page size when iterating through range
// descriptors.
var RangeDescPageSize = settings.RegisterIntSetting(
	settings.ApplicationLevel,
	"server.span_stats.range_desc_page_size",
	"the page size when iterating through range descriptors",
	100,
	settings.IntInRange(5, 25000),
)
