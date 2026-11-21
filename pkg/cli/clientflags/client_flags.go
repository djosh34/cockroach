// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package clientflags

import (
	"fmt"
	"os"

	"github.com/cockroachdb/cockroach/pkg/cli/clienturl"
	"github.com/cockroachdb/cockroach/pkg/cli/cliflagcfg"
	"github.com/cockroachdb/cockroach/pkg/cli/cliflags"
	"github.com/cockroachdb/cockroach/pkg/cli/clisqlcfg"
	"github.com/cockroachdb/cockroach/pkg/security/clientsecopts"
	"github.com/cockroachdb/cockroach/pkg/util/netutil/addr"
	"github.com/spf13/cobra"
)

// AddBaseFlags adds command-line flags common to all client commands.
// This needs to be called before AddSQLFlags().
func AddBaseFlags(
	cmd *cobra.Command,
	clientOpts *clientsecopts.ClientOptions,
	insecureBool *bool,
	configFile *string,
) {
	f := cmd.PersistentFlags()

	// --host/-h.
	// Note that this flag is hidden for SQL commands.
	cliflagcfg.VarFlagDepth(1, f, addr.NewAddrSetter(&clientOpts.ServerHost, &clientOpts.ServerPort), cliflags.ClientHost)

	// The --port/-p flag is supported for backward compatibility with
	// previous versions of CockroachDB. We also need the flag to be
	// defined for the env var override to work properly.
	cliflagcfg.StringFlagDepth(1, f, &clientOpts.ServerPort, cliflags.ClientPort)
	_ = f.MarkHidden(cliflags.ClientPort.Name)

	// --insecure.
	// NB: Insecure is deprecated. See #53404.
	cliflagcfg.BoolFlagDepth(1, f, insecureBool, cliflags.ClientInsecure)

	// --config.
	cliflagcfg.StringFlagDepth(1, f, configFile, cliflags.ConfigFile)
}

// AddSQLFlags adds command-line flags common to SQL client
// commands.
// This needs to be called after AddBaseFlags().
func AddSQLFlags(
	cmd *cobra.Command,
	clientOpts *clientsecopts.ClientOptions,
	sqlCfg *clisqlcfg.Context,
	isShell bool,
	isDemo bool,
) {
	f := cmd.PersistentFlags()

	// The --echo-sql flag is special: it is a marker for CLI tests to
	// recognize SQL-only commands. If/when adding this flag to non-SQL
	// commands, ensure the cli.isSQLCommand() predicate is updated accordingly.
	cliflagcfg.BoolFlagDepth(1, f, &sqlCfg.ConnCtx.Echo, cliflags.EchoSQL)

	// We promote --url as the main way to run SQL clients.
	// --host is supported for backward-compatibility and convenience
	// but we don't really want it to show up in docs.
	_ = f.MarkHidden(cliflags.ClientHost.Name)

	// --url.
	warnFn := func(format string, args ...interface{}) {
		fmt.Fprintf(os.Stderr, format, args...)
	}
	cliflagcfg.VarFlagDepth(1, f, clienturl.NewURLParser(cmd, clientOpts, false /* strictTLS */, warnFn), cliflags.URL)

	// --user/-u
	cliflagcfg.StringFlagDepth(1, f, &clientOpts.User, cliflags.User)

	if isShell || isDemo {
		// --database/-d
		cliflagcfg.StringFlagDepth(1, f, &clientOpts.Database, cliflags.Database)

		// --set
		cliflagcfg.VarFlagDepth(1, f, &sqlCfg.ShellCtx.SetStmts, cliflags.Set)
		// --execute/-e
		cliflagcfg.VarFlagDepth(1, f, &sqlCfg.ShellCtx.ExecStmts, cliflags.Execute)
		// --file/-f
		cliflagcfg.StringFlagDepth(1, f, &sqlCfg.InputFile, cliflags.File)
		// --watch
		cliflagcfg.DurationFlagDepth(1, f, &sqlCfg.ShellCtx.RepeatDelay, cliflags.Watch)
		// --safe-updates
		cliflagcfg.VarFlagDepth(1, f, &sqlCfg.SafeUpdates, cliflags.SafeUpdates)
		// The "safe-updates" flag is tri-valued (true, false, not-specified).
		// If the flag is specified on the command line, but is not given a value,
		// then use the value "true".
		f.Lookup(cliflags.SafeUpdates.Name).NoOptDefVal = "true"

		// --no-line-editor
		cliflagcfg.BoolFlagDepth(1, f, &sqlCfg.ShellCtx.DisableLineEditor, cliflags.NoLineEditor)

		// --read-only
		cliflagcfg.BoolFlagDepth(1, f, &sqlCfg.ReadOnly, cliflags.ReadOnly)

		// --debug-sql-cli
		cliflagcfg.BoolFlagDepth(1, f, &sqlCfg.ConnCtx.DebugMode, cliflags.CliDebugMode)
		// --embedded
		cliflagcfg.BoolFlagDepth(1, f, &sqlCfg.CliCtx.EmbeddedMode, cliflags.EmbeddedMode)
	}

	if isDemo {
		// The 'demo' command does not really support --url or --user.
		// However, we create the pflag instance so that the user
		// can use \connect inside the shell session.
		_ = f.MarkHidden(cliflags.URL.Name)
		_ = f.MarkHidden(cliflags.User.Name)
		// As above, 'demo' does not really support --database.
		// However, we create the pflag instance so that
		// the user can use \connect inside the shell.
		_ = f.MarkHidden(cliflags.Database.Name)
	}
}
