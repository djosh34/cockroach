// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package server

import (
	"context"
	"net/http"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/stretchr/testify/require"
)

func TestNoFallbackParam(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	ctx := context.Background()
	s := serverutils.StartServerOnly(t, base.TestServerArgs{})
	defer s.Stopper().Stop(ctx)

	httpClient, err := s.SystemLayer().GetUnauthenticatedHTTPClient()
	require.NoError(t, err)
	defer httpClient.CloseIdleConnections()

	endpoint := s.SystemLayer().AdminURL().String() + "/health"
	t.Run("requests for a non-existent tenant return an error when fallback is true", func(t *testing.T) {
		resp, err := httpClient.Get(endpoint + "?cluster=doesnotexist&nofallback=true")
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, resp.StatusCode, http.StatusServiceUnavailable)
	})
	t.Run("requests for a non-existent tenant fall backs to default tenant when fallback is false", func(t *testing.T) {
		resp, err := httpClient.Get(endpoint + "?cluster=doesnotexist&nofallback=false")
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, resp.StatusCode, http.StatusOK)
	})
	t.Run("requests for a non-existent tenant fall backs to default tenant when fallback is not specified", func(t *testing.T) {
		resp, err := httpClient.Get(endpoint + "?cluster=doesnotexist")
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, resp.StatusCode, http.StatusOK)
	})
}
