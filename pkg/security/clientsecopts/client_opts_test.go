// Copyright 2026 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package clientsecopts_test

import (
	"testing"

	"github.com/cockroachdb/cockroach/pkg/security/clientsecopts"
	"github.com/cockroachdb/cockroach/pkg/server/pgurl"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/stretchr/testify/require"
)

func TestUsesConfigClientCert(t *testing.T) {
	defer leaktest.AfterTest(t)()
	parse := func(s string) *pgurl.URL {
		u, err := pgurl.Parse(s)
		require.NoError(t, err)
		return u
	}
	require.True(t, clientsecopts.UsesConfigClientCert(false, nil))
	require.False(t, clientsecopts.UsesConfigClientCert(true, nil))
	require.True(t, clientsecopts.UsesConfigClientCert(false, parse("postgresql://h:26257?sslmode=verify-full")))
	require.False(t, clientsecopts.UsesConfigClientCert(false, parse("postgresql://h:26257?sslmode=disable")))
	require.False(t, clientsecopts.UsesConfigClientCert(false,
		parse("postgresql://h:26257?sslmode=verify-full&sslcert=c.crt&sslkey=c.key")))
}
