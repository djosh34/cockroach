// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package compengine

import (
	"fmt"
	"strings"
	"testing"
	"text/tabwriter"

	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/datadriven"
)

// TestCompletionsSketch tests the completionSketch() function,
// as well as the resulting relative indexing of tokens/markers.
func TestCompletionSketch(t *testing.T) {
	defer leaktest.AfterTest(t)()

	datadriven.RunTest(t, "testdata/completion_sketch", func(t *testing.T, td *datadriven.TestData) string {
		switch td.Cmd {
		case "comp":
			offset := 0
			if td.HasArg("at") {
				td.ScanArgs(t, "at", &offset)
			}

			tokens, sketch, pos := CompletionSketch(td.Input, offset)
			if err := CheckSketch(tokens, sketch, pos); err != nil {
				t.Fatalf("%s: %v", td.Pos, err)
			}
			var buf strings.Builder

			tw := tabwriter.NewWriter(&buf, 2, 1, 2, ' ', 0)
			for idx := range sketch {
				quoted := byte(' ')
				if tokens[idx].Quoted {
					quoted = 'q'
				}
				tok := tokens[idx]
				fmt.Fprintf(&buf, "%c\t%q\t%c\t%d\t%d\n", sketch[idx], tok.Str, quoted, tok.Start, tok.End)
			}

			fmt.Fprintln(tw, "--")
			// Now print just a few tokens surrounding the cursor position.
			c := completions{tokens: tokens, sketch: sketch, queryTokIdx: pos}
			for i := -2; i <= 2; i++ {
				tok := c.RelToken(i)
				quoted := byte(' ')
				if tok.Quoted {
					quoted = 'q'
				}
				fmt.Fprintf(&buf, "%d\t%q\t%c\t%d\t%d\n", i, tok.Str, quoted, tok.Start, tok.End)
			}

			_ = tw.Flush()
			return buf.String()

		default:
			t.Fatalf("%s: unknown command: %s", td.Pos, td.Cmd)
		}
		return ""
	})
}
