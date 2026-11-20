// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package kvpb

import (
	"context"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
)

func TestNewDecommissionedStatusErrorf(t *testing.T) {
	ctx := context.Background()
	err := errors.Wrap(NewDecommissionedStatusErrorf(codes.Unauthenticated, "hello %s", "world"), "!")
	require.True(t, IsDecommissionedStatusErr(err))
	ee := errors.EncodeError(ctx, err)
	require.True(t, IsDecommissionedStatusErr(errors.DecodeError(ctx, ee)))
}
