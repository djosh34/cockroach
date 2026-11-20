// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

// Package grunning is a library that's able to retrieve on-CPU running time for
// individual goroutines. It needs a patched Go runtime, which this build does
// not use: stock Go has no per-goroutine running time, so Supported returns
// false and Time returns 0. Callers fall back to other signals.
package grunning

import "time"

// Time returns the time spent by the current goroutine in the running state.
func Time() time.Duration {
	return time.Duration(grunningnanos())
}

// Difference is a helper function to compute the absolute difference between
// two durations.
func Difference(a, b time.Duration) time.Duration {
	diff := a.Nanoseconds() - b.Nanoseconds()
	if diff < 0 {
		diff = -diff
	}
	return time.Duration(diff)
}

// Elapsed returns the running time spent doing some piece of work, with
// grunning.Time() measurements from the start and end.
//
// NB: This only exists due to grunning.Time()'s non-monotonicity, a bug in our
// runtime patch: https://github.com/cockroachdb/cockroach/issues/95529. We can
// get rid of this, keeping just grunning.Difference(), if that bug is fixed.
// The bug results in slight {over,under}-estimation of the running time (the
// latter breaking monotonicity), but is livable with our current uses of this
// library.
func Elapsed(start, end time.Duration) time.Duration {
	diff := end.Nanoseconds() - start.Nanoseconds()
	if diff < 0 {
		diff = 0
	}
	return time.Duration(diff)
}

// Supported returns true iff per-goroutine running time is available in this
// build. It is always false on stock Go.
func Supported() bool {
	return supported()
}
