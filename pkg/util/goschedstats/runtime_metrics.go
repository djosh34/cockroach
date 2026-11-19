// Copyright 2026 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

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
