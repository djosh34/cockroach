// Copyright 2026 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/backup"
	"github.com/cockroachdb/cockroach/pkg/security/certconfig"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/stretchr/testify/require"
)

const testBackupSection = `
backup:
  bucket: b
  full-schedule: "@daily"
  keep-full: 1
`

// TestInsecureNodeLoadsBackupSection checks that an insecure node takes the
// backup section of --config and ignores the rest.
func TestInsecureNodeLoadsBackupSection(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)
	defer initCLIDefaults()

	config := filepath.Join(t.TempDir(), "config")
	// The cert files do not exist; an insecure node does not read them.
	require.NoError(t, os.WriteFile(config, []byte("cert: missing.crt\n"+testBackupSection), 0600))
	initCLIDefaults()
	require.NoError(t, startCmd.Flags().Parse([]string{"--insecure", "--config", config}))
	require.NoError(t, extraServerFlagInit(startCmd))
	require.NotNil(t, serverCfg.Certs.Backup)
	require.Equal(t, "b", serverCfg.Certs.Backup.Bucket)
	require.Empty(t, serverCfg.Certs.Cert)
}

// TestBackupCommandsReadOnlyTheBackupSection checks that backup info,
// check and expire do not need the certificate files of --config.
func TestBackupCommandsReadOnlyTheBackupSection(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)
	defer initCLIDefaults()

	config := filepath.Join(t.TempDir(), "config")
	require.NoError(t, os.WriteFile(config, []byte(
		"client-cert: missing.crt\nclient-key: missing.key\nrules: missing\n"+testBackupSection), 0600))
	initCLIDefaults()
	require.NoError(t, backupInfoCmd.Flags().Parse([]string{"--config", config}))
	require.NoError(t, extraClientFlagInit())
	var bucket string
	require.NoError(t, withBackupStore(func(_ context.Context, _ *backup.Store, b *certconfig.Backup) error {
		bucket = b.Bucket
		return nil
	}))
	require.Equal(t, "b", bucket)
}
