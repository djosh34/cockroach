// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package apiconstants

const (
	// APIV2Path is the prefix for the RESTful v2 API.
	APIV2Path = "/api/v2/"

	// AdminPrefix is the prefix for RESTful endpoints used to provide an
	// administrative interface to the cockroach cluster.
	AdminPrefix = "/_admin/v1/"

	// AdminHealth is the canonical URL path to the health endpoint.
	// (This is also aliased via /health.)
	AdminHealth = AdminPrefix + "health"

	// StatusPrefix is the root of the cluster statistics and metrics API.
	StatusPrefix = "/_status/"

	// StatusVars exposes Prometheus metrics for monitoring consumption.
	StatusVars = StatusPrefix + "vars"

	// LoadStatusVars exposes prometheus metrics for instant monitoring of CPU load.
	LoadStatusVars = StatusPrefix + "load"

	// DefaultAPIEventLimit is the default maximum number of events
	// returned by any endpoints returning events.
	DefaultAPIEventLimit = 1000

	// MaxConcurrentRequests is the maximum number of RPC fan-out requests
	// that will be made at any point of time.
	MaxConcurrentRequests = 100

	// MaxConcurrentPaginatedRequests is the maximum number of RPC fan-out
	// requests that will be made at any point of time for a row-limited /
	// paginated request. This should be much lower than maxConcurrentRequests
	// as too much concurrency here can result in wasted results.
	MaxConcurrentPaginatedRequests = 4
)
