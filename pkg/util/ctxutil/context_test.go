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

import (
	"context"
	"testing"
	"time"

	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/stretchr/testify/require"
)

func TestWhenDone(t *testing.T) {
	defer leaktest.AfterTest(t)()

	makeDoneWatcher := func(t *testing.T) (whenDone func(), waitDone func()) {
		done := make(chan struct{})
		return func() { close(done) }, func() {
			select {
			case <-done:
			case <-time.After(30 * time.Second):
				t.Fatal("timeout")
			}
		}
	}

	t.Run("whenDone", func(t *testing.T) {
		parent, cancelParent := context.WithCancel(context.Background())
		whenDone, waitDone := makeDoneWatcher(t)
		require.True(t, WhenDone(parent, whenDone))
		cancelParent()
		waitDone()
	})

	t.Run("whenDoneAlreadyCancelled", func(t *testing.T) {
		parent, cancelParent := context.WithCancel(context.Background())
		cancelParent() // immediately cancel parent.
		whenDone, waitDone := makeDoneWatcher(t)
		require.True(t, WhenDone(parent, whenDone))
		waitDone()
	})

	t.Run("nonCancellable", func(t *testing.T) {
		require.False(t, WhenDone(&noOpCtx{}, func() { t.Fatal("unexpected call") }))
	})
}

type noOpCtx struct{}

var _ context.Context = (*noOpCtx)(nil)

func (n *noOpCtx) Deadline() (deadline time.Time, ok bool) {
	return time.Time{}, false
}

func (n *noOpCtx) Done() <-chan struct{} {
	return nil
}

func (n *noOpCtx) Err() error {
	return nil
}

func (n *noOpCtx) Value(key any) any {
	return nil
}
