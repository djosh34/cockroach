// Copyright 2026 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package server

import (
	"testing"

	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/stretchr/testify/require"
)

// TestRedactStatementDropsPlaceholders checks that redacted events, which
// debug zip collects, leave out placeholder values: they can hold passwords.
func TestRedactStatementDropsPlaceholders(t *testing.T) {
	defer leaktest.AfterTest(t)()
	info := redactStatement(`{"Statement": "CREATE ROLE r WITH PASSWORD $1", "PlaceholderValues": ["'SECRETPW'"], "Tag": "CREATE ROLE"}`)
	require.NotContains(t, info, "SECRETPW")
	require.Contains(t, info, "CREATE ROLE")
}
