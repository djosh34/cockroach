// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package pgwire

import (
	"testing"

	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/cockroach/pkg/sql/pgwire/hba"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/stretchr/testify/require"
)

func createAuthPipe() *authPipe {
	return newAuthPipe(
		nil,   /* c */
		false, /* logAuthn */
		authOptions{
			connType: hba.ConnLocal,
		},
		username.SQLUsername{},
	)
}

func TestNoMorePwdDataIdempotent(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	ap := createAuthPipe()
	ap.noMorePwdData()
	ap.noMorePwdData()
}

func TestGetPwdDataAfterNoMorePwdData(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	ap := createAuthPipe()
	ap.noMorePwdData()
	// GetPwdData should not hang waiting for p.writerDone to close.
	_, err := ap.GetPwdData()
	require.ErrorContains(t, err, writerDoneError)
}
