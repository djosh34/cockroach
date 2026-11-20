// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package srverrors_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/server/srverrors"
	"github.com/cockroachdb/cockroach/pkg/sql/pgwire/pgcode"
	"github.com/cockroachdb/cockroach/pkg/sql/pgwire/pgerror"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/stretchr/testify/require"
)

func TestServerError(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)
	ctx := context.Background()
	pgError := pgerror.New(pgcode.OutOfMemory, "TestServerError.OutOfMemory")
	err := srverrors.ServerError(ctx, pgError)
	require.Equal(t, "rpc error: code = Internal desc = An internal server error has occurred. Please check your patchroaddb logs for more details. Error Code: 53200", err.Error())

	err = srverrors.ServerError(ctx, err)
	require.Equal(t, "rpc error: code = Internal desc = An internal server error has occurred. Please check your patchroaddb logs for more details. Error Code: 53200", err.Error())

	err = fmt.Errorf("random error that is not pgerror or grpcstatus")
	err = srverrors.ServerError(ctx, err)
	require.Equal(t, "rpc error: code = Internal desc = An internal server error has occurred. Please check your patchroaddb logs for more details.", err.Error())
}
