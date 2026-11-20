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
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/syncutil"
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

// hostRecorder is an http.RoundTripper that records the host of each request
// and answers it with 403 Forbidden, which is not retried.
type hostRecorder struct {
	mu    syncutil.Mutex
	hosts []string
}

func (h *hostRecorder) RoundTrip(req *http.Request) (*http.Response, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.hosts = append(h.hosts, req.URL.Host)
	return &http.Response{
		StatusCode: http.StatusForbidden,
		Header:     http.Header{},
		Body:       io.NopCloser(strings.NewReader("")),
		Request:    req,
	}, nil
}

// TestAssumeRoleUsesRegion checks that STS is called in the region of the URI
// or, without one, in the region of the environment.
func TestAssumeRoleUsesRegion(t *testing.T) {
	defer leaktest.AfterTest(t)()
	ctx := context.Background()
	t.Setenv("AWS_REGION", "ca-central-1")
	for _, tc := range []struct{ region, wantHost string }{
		{"eu-west-1", "sts.eu-west-1.amazonaws.com"},
		{"", "sts.ca-central-1.amazonaws.com"},
	} {
		t.Run(tc.wantHost, func(t *testing.T) {
			recorder := &hostRecorder{}
			cfg, err := loadAWSConfig(ctx, awsConfigParams{
				accessKey:          "key",
				secret:             "secret",
				region:             tc.region,
				assumeRoleProvider: roleProvider{roleARN: "arn:aws:iam::123456789012:role/r"},
				httpClient:         &http.Client{Transport: recorder},
			})
			require.NoError(t, err)
			_, err = cfg.Credentials.Retrieve(ctx)
			require.Error(t, err)
			recorder.mu.Lock()
			defer recorder.mu.Unlock()
			require.NotEmpty(t, recorder.hosts)
			for _, host := range recorder.hosts {
				require.Equal(t, tc.wantHost, host)
			}
		})
	}
}
