// Copyright 2020 The Cockroach Authors.
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
	"strings"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/cli/democluster"
	"github.com/cockroachdb/cockroach/pkg/testutils/datapathutils"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/datadriven"
)

func TestDemo(t *testing.T) {
	defer leaktest.AfterTest(t)()
	c := NewCLITest(TestCLIParams{T: t, NoServer: true})
	defer c.Cleanup()

	defer democluster.TestingForceRandomizeDemoPorts()()

	setCLIDefaultsForTests()

	// Using datadriven allows TESTFLAGS=-rewrite.
	datadriven.RunTest(t, datapathutils.TestDataPath(t, "demo", "test_demo"), func(t *testing.T, td *datadriven.TestData) string {
		if td.Cmd != "exec" {
			t.Fatalf("unsupported command: %s", td.Cmd)
		}
		cmd := strings.Split(td.Input, "\n")
		// Disable multi-tenant for this test due to the unsupported gossip commands.
		cmd = append(cmd, "--multitenant=false")
		cmd = append(cmd, "--logtostderr")
		log.TestingResetActive()
		out, err := c.RunWithCaptureArgs(cmd)
		if err != nil {
			t.Fatal(err)
		}
		// Skip the first line, since that just echoes the command.
		_, afterFirstLine, _ := strings.Cut(out, "\n")
		return afterFirstLine
	})
}
