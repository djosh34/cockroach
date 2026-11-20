// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.
//

package grpcutil

import (
	"testing"

	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/errors"
	"github.com/cockroachdb/redact"
	"github.com/gogo/status"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	grpcStatus "google.golang.org/grpc/status"
)

func TestGRPCErrRedaction(t *testing.T) {
	defer leaktest.AfterTest(t)()
	t.Run("prints gogo status errors", func(t *testing.T) {
		s := status.Newf(codes.Unauthenticated, "%d %s %s", 1, "two", redact.Safe("three"))
		err := errors.Wrap(s.Err(), "boom")
		require.EqualValues(t, `boom: grpc: ‹1 two three› [code 16/Unauthenticated]`, redact.Sprint(err))
	})
	t.Run("does not handle nil status.Status", func(t *testing.T) {
		e := &testingErrWithGRPCStatus{}
		err := errors.Wrap(e, "boom")
		require.EqualValues(t, `boom: ‹test error›`, redact.Sprint(err))
	})
}

type testingErrWithGRPCStatus struct{}

func (e *testingErrWithGRPCStatus) GRPCStatus() *grpcStatus.Status { return nil }
func (e *testingErrWithGRPCStatus) Error() string                  { return "test error" }
