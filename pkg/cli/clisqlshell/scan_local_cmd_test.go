// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package clisqlshell

import (
	"fmt"
	"strings"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/cli/clierror"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/datadriven"
)

func TestScanLocalCmdArgs(t *testing.T) {
	defer leaktest.AfterTest(t)()

	datadriven.Walk(t, "testdata/local_cmds", func(t *testing.T, path string) {
		datadriven.RunTest(t, path, func(t *testing.T, td *datadriven.TestData) string {
			switch td.Cmd {
			case "run":
				var buf strings.Builder
				args, err := scanLocalCmdArgs(td.Input)
				if err != nil {
					clierror.OutputError(&buf, err, true, false)
					return buf.String()
				}
				fmt.Fprintf(&buf, "%d args:\n", len(args))
				for _, arg := range args {
					fmt.Fprintf(&buf, "%q\n", arg)
				}
				return buf.String()

			default:
				t.Fatalf("unknown command: %s", td.Cmd)
				return "unreachable"
			}
		})
	})
}
