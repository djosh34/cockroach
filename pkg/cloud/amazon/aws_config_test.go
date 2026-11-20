// Copyright 2026 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package amazon

import (
	"context"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/stretchr/testify/require"
)

// TestLoadAWSConfigIgnoresProfileForURICredentials checks that credentials in
// the URI work when the node's environment names a profile that its AWS files
// do not have.
func TestLoadAWSConfigIgnoresProfileForURICredentials(t *testing.T) {
	defer leaktest.AfterTest(t)()
	t.Setenv("AWS_PROFILE", "profile-that-does-not-exist")
	ctx := context.Background()
	cfg, err := loadAWSConfig(ctx, awsConfigParams{accessKey: "key", secret: "secret"})
	require.NoError(t, err)
	creds, err := cfg.Credentials.Retrieve(ctx)
	require.NoError(t, err)
	require.Equal(t, "key", creds.AccessKeyID)
}
