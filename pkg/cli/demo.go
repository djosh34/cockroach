// Copyright 2018 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cockroachdb/cockroach/pkg/cli/clienturl"
	"github.com/cockroachdb/cockroach/pkg/cli/clierrorplus"
	"github.com/cockroachdb/cockroach/pkg/cli/cliflagcfg"
	"github.com/cockroachdb/cockroach/pkg/cli/cliflags"
	"github.com/cockroachdb/cockroach/pkg/cli/clisqlclient"
	"github.com/cockroachdb/cockroach/pkg/cli/democluster"
	"github.com/cockroachdb/cockroach/pkg/server/serverpb"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/util"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/stop"
	"github.com/cockroachdb/cockroach/pkg/workload"
	"github.com/cockroachdb/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var demoCmd = &cobra.Command{
	Use:   "demo",
	Short: "open a demo sql shell",
	Long: `
Start an in-memory, standalone, single-node CockroachDB instance, and open an
interactive SQL prompt to it. Various datasets are available to be preloaded as
subcommands: e.g. "cockroach demo startrek". See --help for a full list.

By default, the 'movr' dataset is pre-loaded. You can also use --no-example-database
to avoid pre-loading a dataset.

cockroach demo attempts to connect to a Cockroach Labs server to send
telemetry back to Cockroach Labs. In order to disable this behavior, set the
environment variable "COCKROACH_SKIP_ENABLING_DIAGNOSTIC_REPORTING" to true.
`,
	Example: `  cockroach demo`,
	Args:    cobra.NoArgs,
	// Note: RunE is set in the init() function below to avoid an
	// initialization cycle.
}

func init() {
	demoCmd.RunE = clierrorplus.MaybeDecorateError(func(cmd *cobra.Command, _ []string) error {
		return runDemo(cmd, nil /* gen */)
	})
}

const defaultGeneratorName = "movr"

var defaultGenerator workload.Generator

func init() {
	for _, meta := range workload.Registered() {
		gen := meta.New()

		if meta.Name == defaultGeneratorName {
			// Save the default for use in the top-level 'demo' command
			// without argument.
			defaultGenerator = gen
		}

		var genFlags *pflag.FlagSet
		if f, ok := gen.(workload.Flagser); ok {
			genFlags = f.Flags().FlagSet
		}

		genDemoCmd := &cobra.Command{
			Use:   meta.Name,
			Short: meta.Description,
			Long:  meta.Description + meta.Details,
			Args:  cobra.ArbitraryArgs,
			RunE: clierrorplus.MaybeDecorateError(func(cmd *cobra.Command, _ []string) error {
				return runDemo(cmd, gen)
			}),
		}
		if meta.TestInfraOnly {
			demoCmd.Long = "THIS COMMAND WAS DEVELOPED FOR INTERNAL TESTING ONLY.\n\n" + demoCmd.Long
		}
		demoCmd.AddCommand(genDemoCmd)
		genDemoCmd.Flags().AddFlagSet(genFlags)
	}
}

func incrementTelemetryCounters(cmd *cobra.Command) {
	incrementDemoCounter(demo)
	if cliflagcfg.FlagSetForCmd(cmd).Lookup(cliflags.DemoNodes.Name).Changed {
		incrementDemoCounter(nodes)
	}
	if demoCtx.Localities != nil {
		incrementDemoCounter(demoLocality)
	}
	if demoCtx.RunWorkload {
		incrementDemoCounter(withLoad)
	}
}

func checkDemoConfiguration(
	cmd *cobra.Command, gen workload.Generator,
) (workload.Generator, error) {
	if gen == nil && !demoCtx.UseEmptyDatabase {
		// Use a default dataset unless prevented by --no-example-database.
		gen = defaultGenerator
	}

	// Make sure that the user didn't request a workload and an empty database.
	if demoCtx.RunWorkload && demoCtx.UseEmptyDatabase {
		return nil, errors.New("cannot run a workload when generation of the example database is disabled")
	}

	// Make sure the number of nodes is valid.
	if demoCtx.NumNodes <= 0 {
		return nil, errors.Newf("--%s has invalid value (expected positive, got %d)", cliflags.DemoNodes.Name, demoCtx.NumNodes)
	}

	// If artificial latencies were requested, then the user cannot supply their own localities.
	if demoCtx.SimulateLatency && demoCtx.Localities != nil {
		return nil, errors.Newf("--%s cannot be used with --%s", cliflags.Global.Name, cliflags.DemoNodeLocality.Name)
	}

	return gen, nil
}

func runDemo(cmd *cobra.Command, gen workload.Generator) error {
	return runDemoInternal(cmd, gen, func(context.Context, clisqlclient.Conn) error { return nil })
}

func runDemoInternal(
	cmd *cobra.Command,
	gen workload.Generator,
	extraInit func(context.Context, clisqlclient.Conn) error,
) (resErr error) {
	closeFn, err := sqlCtx.Open(os.Stdin)
	if err != nil {
		return err
	}
	defer closeFn()

	if gen, err = checkDemoConfiguration(cmd, gen); err != nil {
		return err
	}
	// Record some telemetry about what flags are being used.
	incrementTelemetryCounters(cmd)

	ctx := context.Background()

	demoCtx.WorkloadGenerator = gen

	c, err := democluster.NewDemoCluster(ctx, &demoCtx.Context,
		log.Infof,
		log.Warningf,
		log.Ops.Shoutf,
		func(ctx context.Context) (*stop.Stopper, error) {
			// Override the default server store spec.
			//
			// This is needed because the logging setup code peeks into this to
			// decide how to enable logging.
			serverCfg.Stores.Specs = nil
			return setupAndInitializeLoggingAndProfiling(ctx, cmd, false /* isServerCmd */)
		},
		func(ctx context.Context, ac serverpb.AdminClient) error {
			return drainAndShutdown(ctx, ac, "local" /* targetNode */)
		},
	)
	if err != nil {
		c.Close(ctx)
		return err
	}
	defer c.Close(ctx)

	if err := c.Start(ctx); err != nil {
		return clierrorplus.CheckAndMaybeShout(err)
	}
	sqlCtx.ShellCtx.DemoCluster = c

	if demoCtx.DefaultEnableRangefeeds {
		if err = c.SetClusterSetting(ctx, "kv.rangefeed.enabled", true); err != nil {
			return clierrorplus.CheckAndMaybeShout(err)
		}
	}

	if cliCtx.IsInteractive {
		cliCtx.PrintfUnlessEmbedded(`#
# Welcome to the CockroachDB demo database!
#
# You are connected to a temporary, in-memory CockroachDB cluster of %d node%s.
`, demoCtx.NumNodes, util.Pluralize(int64(demoCtx.NumNodes)))

		if demoCtx.Multitenant {
			cliCtx.PrintfUnlessEmbedded(`#
# You are connected to tenant %q, but can connect to the system tenant with
# '\connect' and the SQL url printed via '\demo ls'.
`, c.TenantName())
		}

		if demoCtx.SimulateLatency {
			cliCtx.PrintfUnlessEmbedded(
				`# Communication between nodes will simulate real world latencies.
#
# WARNING: the use of --%s is experimental. Some features may not work as expected.
`,
				cliflags.Global.Name,
			)
		}

		// Only print details about the telemetry configuration if the
		// user has control over it.
		if cluster.TelemetryOptOut {
			cliCtx.PrintlnUnlessEmbedded("#\n# Telemetry disabled by configuration.")
		} else {
			cliCtx.PrintlnUnlessEmbedded("#\n" +
				"# This demo session will send telemetry to Cockroach Labs in the background.\n" +
				"# To disable this behavior, set the environment variable\n" +
				"# COCKROACH_SKIP_ENABLING_DIAGNOSTIC_REPORTING=true.")
		}
	}

	// Initialize the workload, if requested.
	if err := c.SetupWorkload(ctx); err != nil {
		return clierrorplus.CheckAndMaybeShout(err)
	}

	if cliCtx.IsInteractive {
		if gen != nil {
			fmt.Printf("#\n# The cluster has been preloaded with the %q dataset\n# (%s).\n",
				gen.Meta().Name, gen.Meta().Description)
		}

		fmt.Println(`#
# Reminder: your changes to data stored in the demo session will not be saved!`)

		var nodeList strings.Builder
		c.ListDemoNodes(&nodeList, stderr, true /* justOne */, false /* verbose */)
		cliCtx.PrintlnUnlessEmbedded(
			// Only print the server details when the shell is not embedded;
			// if embedded, the embedding platform owns the network
			// configuration.
			`#
# If you wish to access this demo cluster using another tool, you will need
# the following details:
#
#   - Connection parameters:
#  `,
			strings.ReplaceAll(strings.TrimSuffix(nodeList.String(), "\n"), "\n", "\n#   "))

		if !demoCtx.Insecure {
			adminUser, adminPassword, certsDir := c.GetSQLCredentials()

			fmt.Printf(`#   - Username: %q, password: %q
#   - Directory with certificate files (for certain SQL drivers/tools): %s
#
# You can enter \info to print these details again.
#
`,
				adminUser,
				adminPassword,
				certsDir,
			)
		}
	}

	conn, err := sqlCtx.MakeConn(c.GetConnURL())
	if err != nil {
		return err
	}
	defer func() { resErr = errors.CombineErrors(resErr, conn.Close()) }()

	_, _, certsDir := c.GetSQLCredentials()
	sqlCtx.ShellCtx.CertsDir = certsDir
	sqlCtx.ShellCtx.ParseURL = clienturl.MakeURLParserFn(cmd, cliCtx.clientOpts)

	if err := extraInit(ctx, conn); err != nil {
		return err
	}

	// Report the PID of the process in a configured PID file. Used in tests.
	if demoCtx.pidFile != "" {
		if err := os.WriteFile(demoCtx.pidFile, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0644); err != nil {
			return err
		}
	}

	// Enable the latency as late in the process of starting the cluster as we
	// can to minimize the startup time.
	if demoCtx.SimulateLatency {
		c.SetSimulatedLatency(true /* on */)
		defer c.SetSimulatedLatency(false /* on */)
	}

	// Ensure the last few entries in the log files are flushed at the end.
	defer log.FlushFiles()

	return sqlCtx.Run(ctx, conn)
}
