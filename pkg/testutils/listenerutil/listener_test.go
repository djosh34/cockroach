// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package listenerutil

import (
	"context"
	"net"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/util/ctxgroup"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/stop"
	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/require"
)

func TestReusableListener(t *testing.T) {
	defer leaktest.AfterTest(t)()

	ctx := context.Background()
	stopper := stop.NewStopper()
	defer stopper.Stop(ctx)

	lr := NewListenerRegistry()
	defer lr.Close()
	ln := lr.MustGetOrCreate(t, 0)

	// connect accepts from the socket and also connects to it to write and then
	// read a single byte. The returned error is that of the connecting goroutine;
	// an error in Accept() is ignored (but others are returned).
	connect := func() error {
		return ctxgroup.GoAndWait(ctx,
			func(ctx context.Context) (rerr error) {
				defer func() {
					rerr = errors.Wrap(rerr, "connecter")
				}()
				c, err := net.Dial("tcp", ln.Addr().String())
				if err != nil {
					return err
				}
				if _, err := c.Write([]byte{'x'}); err != nil {
					return errors.Wrap(err, "write")
				}
				sl := make([]byte, 1)
				if _, err := c.Read(sl); err != nil {
					return errors.Wrap(err, "read")
				}
				if sl[0] != 'x' {
					return errors.Errorf("reply-read: didn't expect %v", sl)
				}
				_ = c.Close()
				return nil
			},
			func(ctx context.Context) (rerr error) {
				defer func() {
					rerr = errors.Wrap(rerr, "accepter")
				}()
				c, err := ln.Accept()
				if err != nil {
					t.Logf("ignoring error from Accept: %s", err)
					return nil
				}
				sl := make([]byte, 1)
				if _, err := c.Read(sl); err != nil {
					return errors.Wrap(err, "read")
				}
				if sl[0] != 'x' {
					return errors.Errorf("read: didn't expect %v", sl)
				}
				if _, err := c.Write([]byte{'x'}); err != nil {
					return errors.Wrap(err, "reply-write")
				}
				_ = c.Close()
				return nil
			},
		)
	}

	require.NoError(t, connect())
	require.NoError(t, ln.Close())
	err := connect()
	require.Error(t, err)
	t.Logf("the expected error is: %v", err)
	require.NoError(t, lr.MustGet(t, 0).Reopen())
	require.NoError(t, connect())
}
