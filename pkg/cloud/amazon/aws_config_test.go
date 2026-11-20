// Copyright 2026 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

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
