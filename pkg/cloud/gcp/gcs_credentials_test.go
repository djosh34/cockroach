// Copyright 2026 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package gcp

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/cloud"
	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/syncutil"
	"github.com/stretchr/testify/require"
)

// TestGCSCredentialsOnlyServiceAccount checks that the CREDENTIALS of a gs URI
// cannot make the node read a local file or call a URL of the user's choice.
// A user who is not an admin may use such a URI in IMPORT or EXPORT.
func TestGCSCredentialsOnlyServiceAccount(t *testing.T) {
	defer leaktest.AfterTest(t)()
	ctx := context.Background()

	var mu syncutil.Mutex
	var received []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		received = append(received, r.URL.Path+" "+string(body))
		mu.Unlock()
		http.Error(w, "no", http.StatusBadRequest)
	}))
	defer srv.Close()

	secretFile := filepath.Join(t.TempDir(), "secret")
	require.NoError(t, os.WriteFile(secretFile, []byte("node-local-secret"), 0600))

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))

	for _, tc := range []struct {
		name  string
		creds map[string]interface{}
	}{
		{
			name: "external account reads a local file",
			creds: map[string]interface{}{
				"type":               "external_account",
				"audience":           "test",
				"subject_token_type": "urn:ietf:params:oauth:token-type:jwt",
				"token_url":          srv.URL + "/token",
				"credential_source":  map[string]interface{}{"file": secretFile},
			},
		},
		{
			name: "service account with a token_uri elsewhere",
			creds: map[string]interface{}{
				"type":           "service_account",
				"client_email":   "test@example.com",
				"private_key_id": "1",
				"private_key":    keyPEM,
				"token_uri":      srv.URL + "/token",
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mu.Lock()
			received = nil
			mu.Unlock()

			credsJSON, err := json.Marshal(tc.creds)
			require.NoError(t, err)
			uri := fmt.Sprintf("gs://bucket/path?AUTH=specified&CREDENTIALS=%s",
				url.QueryEscape(base64.StdEncoding.EncodeToString(credsJSON)))
			conf, err := cloud.ExternalStorageConfFromURI(uri, username.MakeSQLUsernameFromPreNormalizedString("testuser"))
			require.NoError(t, err)
			require.True(t, conf.AccessIsWithExplicitAuth())

			args := cloud.ExternalStorageContext{
				IOConf:          base.ExternalIODirConfig{},
				Settings:        cluster.MakeTestingClusterSettings(),
				MetricsRecorder: cloud.NilMetrics,
			}
			s, err := makeGCSStorage(ctx, args, conf)
			if err == nil {
				// Reading makes the client fetch a token.
				_, _, _ = s.ReadFile(ctx, "file", cloud.ReadOptions{NoFileSize: true})
				_ = s.Close()
			}
			mu.Lock()
			defer mu.Unlock()
			for _, r := range received {
				require.False(t, strings.Contains(r, "node-local-secret"), "the local file reached the server: %s", r)
			}
			require.Empty(t, received, "the node called a URL from the credentials")
			require.ErrorContains(t, err, "credentials")
		})
	}
}
