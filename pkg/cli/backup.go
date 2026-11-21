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
	"fmt"
	"os"
	"time"

	"github.com/cockroachdb/cockroach/pkg/backup"
	"github.com/cockroachdb/cockroach/pkg/cli/clierrorplus"
	"github.com/cockroachdb/cockroach/pkg/security/certconfig"
	"github.com/cockroachdb/cockroach/pkg/util/hlc"
	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"
)

var backupCmd = &cobra.Command{
	Use:   "backup [command]",
	Short: "inspect, check, expire and restore backups",
	Long: `
Commands for the backup repository named in the backup section of --config.
A running cluster takes the full backups and makes the archive itself;
these commands read the repository with the S3 credentials of the
environment (the AWS SDK's default chain).
`,
	RunE: UsageAndErr,
}

var backupInfoCmd = &cobra.Command{
	Use:   "info --config <file>",
	Short: "show the full backups, the archive and the last restorable time",
	Args:  cobra.NoArgs,
	RunE: clierrorplus.MaybeDecorateError(func(cmd *cobra.Command, _ []string) error {
		return withBackupStore(func(ctx context.Context, s *backup.Store, _ *certconfig.Backup) error {
			return backup.Info(ctx, s, os.Stdout)
		})
	}),
}

var backupCheckName string

var backupCheckCmd = &cobra.Command{
	Use:   "check --config <file> [--backup <name>]",
	Short: "verify the checksums of a full backup and its archive",
	Long: `
Reads a full backup, the last one unless --backup names another, and every
archive segment after it, and verifies their checksums and that the
segments follow each other.
`,
	Args: cobra.NoArgs,
	RunE: clierrorplus.MaybeDecorateError(func(cmd *cobra.Command, _ []string) error {
		return withBackupStore(func(ctx context.Context, s *backup.Store, _ *certconfig.Backup) error {
			return backup.Check(ctx, s, backupCheckName, os.Stdout)
		})
	}),
}

var backupExpireCmd = &cobra.Command{
	Use:   "expire --config <file>",
	Short: "delete the full backups and segments that keep-full no longer keeps",
	Long: `
Deletes the full backups older than the newest keep-full ones, the archive
segments only those needed, and full backups that never finished. The
cluster does this itself after each full backup.
`,
	Args: cobra.NoArgs,
	RunE: clierrorplus.MaybeDecorateError(func(cmd *cobra.Command, _ []string) error {
		return withBackupStore(func(ctx context.Context, s *backup.Store, b *certconfig.Backup) error {
			return backup.Expire(ctx, s, b.KeepFull, os.Stdout)
		})
	}),
}

var backupRestoreTarget string

var backupRestoreCmd = &cobra.Command{
	Use:   "restore --config <file> [--target <time>] --host <new cluster>",
	Short: "restore the whole cluster into a new, empty cluster",
	Long: `
Restores the whole cluster from the repository in the backup section of
--config into the cluster that --host or --url names. That cluster must be
new: no databases or tables besides the pre-loaded defaultdb and postgres.
Its nodes read the repository with the S3 credentials of their own
environment.

--target is the time to restore to, as RFC 3339 (2026-11-21T10:00:00Z) or
as the decimal that 'backup info' and cluster_logical_timestamp() show. It
must not be after the last restorable time. Without it, the restore goes to
the last restorable time.

The command connects to SQL with the client certificate of --config and
waits until the restore has finished.
`,
	Args: cobra.NoArgs,
	RunE: clierrorplus.MaybeDecorateError(runBackupRestore),
}

func runBackupRestore(cmd *cobra.Command, _ []string) error {
	return withBackupStore(func(ctx context.Context, s *backup.Store, b *certconfig.Backup) error {
		target, err := parseRestoreTarget(backupRestoreTarget)
		if err != nil {
			return err
		}
		to, err := backup.CheckTarget(ctx, s, target)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "restoring to %s\n", backup.FormatTime(to))
		conn, err := makeSQLClient(ctx, "patchroaddb backup restore", useSystemDb)
		if err != nil {
			return err
		}
		defer func() { _ = conn.Close() }()
		// Send the time printed above, not "latest", which may have moved.
		row, err := conn.QueryRow(ctx, `SELECT crdb_internal.restore_cluster($1, $2, $3, $4, $5)`,
			b.Bucket, b.Prefix, b.Endpoint, b.Region, to.AsOfSystemTime())
		if err != nil {
			return err
		}
		fmt.Printf("restored by job %v\n", row[0])
		return nil
	})
}

// parseRestoreTarget parses --target: empty, RFC 3339 or an HLC decimal.
func parseRestoreTarget(s string) (hlc.Timestamp, error) {
	if s == "" {
		return hlc.Timestamp{}, nil
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return hlc.Timestamp{WallTime: t.UnixNano()}, nil
	}
	ts, err := hlc.ParseHLC(s)
	if err != nil {
		return hlc.Timestamp{}, errors.Newf("--target %q is neither RFC 3339 nor an HLC decimal", s)
	}
	return ts, nil
}

// backupConfigFile is the --config of backup info, check and expire.
var backupConfigFile string

// withBackupStore opens the repository named in the backup section of the
// --config file and calls f.
func withBackupStore(f func(context.Context, *backup.Store, *certconfig.Backup) error) error {
	path := backupConfigFile
	if path == "" {
		// backup restore, a client command.
		path = cliCtx.configFile
	}
	if path == "" {
		return errors.New("--config is required")
	}
	c, err := certconfig.Load(path)
	if err != nil {
		return err
	}
	b := c.Backup
	if b == nil {
		return errors.Newf("%s has no backup section", path)
	}
	ctx := context.Background()
	s, err := backup.OpenStore(ctx, backup.RepositoryOf(b))
	if err != nil {
		return err
	}
	return f(ctx, s, b)
}

var backupCmds = []*cobra.Command{backupInfoCmd, backupCheckCmd, backupExpireCmd, backupRestoreCmd}

func init() {
	backupCmd.AddCommand(backupCmds...)
}
