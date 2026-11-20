// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package schedulerlatency

import "time"

type LatencyObserver interface {
	// SchedulerLatency is provided the current value of the scheduler's p99 latency and the
	// period over which the measurement applies.
	SchedulerLatency(p99 time.Duration, period time.Duration)
}
