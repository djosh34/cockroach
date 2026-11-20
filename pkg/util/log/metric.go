// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package log

// LogMetrics enables the registration and recording of metrics
// within the log package.
//
// Because the log package is imported by nearly every package
// within CRDB, it's difficult to add new dependencies to the
// log package without introducing a circular dependency.
//
// The LogMetrics interface provides us with a way to still
// make use of the metrics library within the log package via
// dependency injection, allowing the implementation to live
// elsewhere (e.g. the metrics package).
type LogMetrics interface {
	// IncrementCounter increments the Counter metric associated with the
	// provided MetricName by the given amount, assuming the
	// metric has been registered.
	//
	// The LogMetrics implementation must have metadata defined
	// for the given MetricName within its own scope. See
	// pkg/util/log/logmetrics for details.
	IncrementCounter(metric Metric, amount int64)
}

// Metric is the enum representation of each metric supported within the log package.
// NB: When adding a metric here, be sure to add to log.Metrics below.
// NB: The metric also needs to be added to pkg/util/log/logmetrics, where we
// need to register the metric and define its metadata.
//
// We use an enum representation as an optimization within LogMetrics implementations
// when doing lookups for underlying metric counters.
type Metric int

const (
	FluentSinkConnectionAttempt Metric = iota
	FluentSinkConnectionError
	FluentSinkWriteAttempt
	FluentSinkWriteError
	BufferedSinkMessagesDropped
	LogMessageCount
)

var Metrics = []Metric{
	FluentSinkConnectionAttempt,
	FluentSinkConnectionError,
	FluentSinkWriteAttempt,
	FluentSinkWriteError,
	BufferedSinkMessagesDropped,
	LogMessageCount,
}
