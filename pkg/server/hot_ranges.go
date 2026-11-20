// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package server

import (
	"time"

	"github.com/cockroachdb/cockroach/pkg/settings"
)

// HotRangesRequestNodeTimeout controls the timeout of a serverpb.HotRangesRequest.
// A default value of 5 minutes is meant to be an escape hatch for a node that is taking
// too long to fulfill the request. The setting should be lowered for a faster response,
// at the expense of possibly incomplete data, or raised for complete data, at the cost
// of a possibly slower response.
var HotRangesRequestNodeTimeout = settings.RegisterDurationSetting(
	settings.ApplicationLevel,
	"server.hot_ranges_request.node.timeout",
	"the duration allowed for a single node to return hot range data before the request is cancelled; if set to 0, there is no timeout",
	time.Minute*5,
	settings.NonNegativeDuration,
	settings.WithPublic)
