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

import "context"

// WhenDoneFunc is the callback invoked by context when it becomes done.
type WhenDoneFunc func()

// WhenDone arranges for the specified function to be invoked when
// parent context becomes done and returns true.
// If the context is non-cancellable (i.e. `Done() == nil`), returns false and
// never calls the function.
//
// It uses context.AfterFunc: if the parent context is derived from
// context.WithCancel or context.WithTimeout/Deadline, no goroutine waits for
// the parent; done runs in its own goroutine once the parent is done.
func WhenDone(parent context.Context, done WhenDoneFunc) bool {
	if parent.Done() == nil {
		return false
	}
	context.AfterFunc(parent, done)
	return true
}
