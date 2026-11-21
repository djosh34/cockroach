// Copyright 2026 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package certconfig_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cockroachdb/cockroach/pkg/security/certconfig"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/stretchr/testify/require"
)

func TestLoadBackupSection(t *testing.T) {
	defer leaktest.AfterTest(t)()
	load := func(t *testing.T, text string) (certconfig.Config, error) {
		p := filepath.Join(t.TempDir(), "config")
		require.NoError(t, os.WriteFile(p, []byte(text), 0600))
		return certconfig.Load(p)
	}

	c, err := load(t, "cert: node.crt\n")
	require.NoError(t, err)
	require.Nil(t, c.Backup)

	c, err = load(t, `
backup:
  bucket: b
  prefix: p
  endpoint: http://s3:7070
  full-schedule: "0 3 * * *"
  keep-full: 2
`)
	require.NoError(t, err)
	require.Equal(t, &certconfig.Backup{
		Bucket: "b", Prefix: "p", Endpoint: "http://s3:7070", FullSchedule: "0 3 * * *",
		ArchiveInterval: certconfig.DefaultArchiveInterval, KeepFull: 2,
	}, c.Backup)

	c, err = load(t, "backup:\n  bucket: b\n  full-schedule: '@every 6h'\n  archive-interval: 10s\n  keep-full: 1\n")
	require.NoError(t, err)
	require.Equal(t, 10*time.Second, c.Backup.ArchiveInterval)

	for _, tc := range []struct{ text, err string }{
		{"backup:\n  full-schedule: '@daily'\n  keep-full: 1\n", "bucket is required"},
		{"backup:\n  bucket: b\n  keep-full: 1\n", "full-schedule is required"},
		{"backup:\n  bucket: b\n  full-schedule: 'not cron'\n  keep-full: 1\n", "parsing full-schedule"},
		{"backup:\n  bucket: b\n  full-schedule: '@daily'\n", "keep-full must be at least 1"},
		{"backup:\n  bucket: b\n  full-schedule: '@daily'\n  keep-full: 1\n  archive-interval: -1s\n", "must be positive"},
		{"backup:\n  bucket: b\n  full-schedule: '@daily'\n  keep-full: 1\n  encryption: yes\n", "field encryption not found"},
	} {
		_, err := load(t, tc.text)
		require.ErrorContains(t, err, tc.err, tc.text)
	}
}
