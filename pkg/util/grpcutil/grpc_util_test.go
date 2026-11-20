// Copyright 2017 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package grpcutil_test

import (
	"fmt"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/server"
	"github.com/cockroachdb/cockroach/pkg/util/circuit"
	"github.com/cockroachdb/cockroach/pkg/util/grpcutil"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/netutil"
	"github.com/cockroachdb/errors"
	"github.com/gogo/status"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
)

func TestIsWaitingForInit(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	testcases := map[string]struct {
		err    error
		expect bool
	}{
		"waiting for init":  {server.NewWaitingForInitError("foo"), true},
		"unavailable error": {status.Errorf(codes.Unavailable, "foo"), false},
		"non-grpc":          {fmt.Errorf("node waiting for init"), false},
	}
	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tc.expect, grpcutil.IsWaitingForInit(tc.err))
		})
	}
}

func TestRequestDidNotStart_Errors(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	testcases := map[string]struct {
		err    error
		expect bool
	}{
		"failed heartbeat":    {&netutil.InitialHeartbeatFailedError{}, true},
		"waiting for init":    {server.NewWaitingForInitError("foo"), true},
		"unauthenticated":     {status.Error(codes.Unauthenticated, "unauthenticated"), true},
		"permission denied":   {status.Error(codes.PermissionDenied, "permission denied"), true},
		"failed precondition": {status.Error(codes.FailedPrecondition, "failed precondition"), true},
		"circuit breaker":     {circuit.ErrBreakerOpen, false},
		"plain":               {errors.New("foo"), false},
	}
	for name, tc := range testcases {
		t.Run(name, func(t *testing.T) {
			// Make sure the error is properly detected both bare and wrapped.
			require.Equal(t, tc.expect, grpcutil.RequestDidNotStart(tc.err))
			require.Equal(t, tc.expect, grpcutil.RequestDidNotStart(errors.Wrap(tc.err, "wrapped")))
		})
	}
}
