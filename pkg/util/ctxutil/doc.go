// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package ctxutil

/*

This package provides context utilities that require some "black magic" to
access internals of context.Context implementation.

In particular, this package provides WhenDone function:

// WhenDone arranges for the specified function to be invoked when
// parent context becomes done and returns true.
func WhenDone(parent context.Context, done WhenDoneFunc) bool {...}


Obviously, one can detect when parent is done and simply invoke the function
when that happens using simple goroutine:
go func() {
  select {
    <-parent.Done()
    invokeCallback()
}()

However, doing so requires spinning up goroutines.  Goroutines are very cheap,
but not 0 cost.  Certain performance critical code should avoid adding additional
goroutines if it can be helped (every goroutine adds just a bit of load on go scheduler,
and when the goroutine is created, it has to run; create enough of these goroutines,
and scheduler will experience high latency).

Hence, the "black magic" in this package.

Currently, there are condition compilation available for go1.19, go1.20, and go1.21.
Once switch to go1.21 completes, earlier version hacks may be removed.
*/
