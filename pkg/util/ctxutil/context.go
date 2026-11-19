// Copyright 2023 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

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
