//go:build sim

package sim

import (
	"net/netip"
	"testing"
	"time"

	"github.com/cockroachdb/cockroach/pkg/cli"
	"github.com/jellevandenhooff/gosim"
)

func TestSpike(t *testing.T) {
	m := gosim.NewMachine(gosim.MachineConfig{
		Label: "n1",
		Addr:  netip.MustParseAddr("10.0.0.1"),
		MainFunc: func() {
			cmd := cli.CockroachCmd()
			cmd.SetArgs([]string{"start-single-node", "--insecure", "--listen-addr=10.0.0.1:26257", "--http-addr=10.0.0.1:8080", "--store=path=/data,ballast-size=0", "--cache=64MiB", "--max-sql-memory=128MiB"})
			if err := cmd.Execute(); err != nil {
				t.Errorf("start: %v", err)
			}
		},
	})
	time.Sleep(30 * time.Second)
	m.Crash()
}
