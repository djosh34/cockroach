// Copyright 2026 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package goschedstats

import (
	"runtime"
	"runtime/metrics"
)

const runnableMetric = "/sched/goroutines/runnable:goroutines"

// numRunnableGoroutines returns the number of goroutines that are ready to run
// but wait for a processor, and GOMAXPROCS. It reads the runtime metric, so it
// works on stock Go.
func numRunnableGoroutines() (numRunnable int, numProcs int) {
	sample := []metrics.Sample{{Name: runnableMetric}}
	metrics.Read(sample)
	if sample[0].Value.Kind() == metrics.KindUint64 {
		numRunnable = int(sample[0].Value.Uint64())
	}
	return numRunnable, runtime.GOMAXPROCS(0)
}
