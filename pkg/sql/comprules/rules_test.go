// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package comprules

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/sql/compengine"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/datadriven"
)

// TestCompletionPatterns checks the traces resulting from running the
// completion heuristics. This validates that the heuristic recognize
// the proper patterns of tokens.
func TestCompletionPatterns(t *testing.T) {
	defer leaktest.AfterTest(t)()

	datadriven.Walk(t, "testdata/completion_patterns", func(t *testing.T, path string) {
		curTraceFilter := regexp.MustCompile(`.*`)
		datadriven.RunTest(t, path, func(t *testing.T, td *datadriven.TestData) string {
			switch td.Cmd {
			case "filter":
				filter, err := regexp.Compile(td.Input)
				if err != nil {
					t.Fatalf("%s: %v", td.Pos, err)
				}
				curTraceFilter = filter

			case "comp":
				offset := 0
				if td.HasArg("at") {
					td.ScanArgs(t, "at", &offset)
				}

				ctx := context.Background()
				var buf strings.Builder
				fakeQueryFn := func(_ context.Context, label string, q string, args ...interface{}) (compengine.Rows, error) {
					if curTraceFilter.MatchString(label + ":") {
						fmt.Fprintf(&buf, "--sql:\n%s\n--placeholders: %#v\n",
							strings.TrimSpace(q), args)
					}
					return nil, nil
				}
				c := compengine.New(fakeQueryFn, GetCompMethods(), offset, td.Input)
				defer c.Close(ctx)

				sketch := c.(interface{ Sketch() string }).Sketch()
				pos := c.(interface{ TestingQueryTokIdx() int }).TestingQueryTokIdx()

				if err := c.(interface{ TestingCheckSketch() error }).TestingCheckSketch(); err != nil {
					t.Fatalf("%s: %v", td.Pos, err)
				}

				fmt.Fprintln(&buf, sketch)
				fmt.Fprintf(&buf, "%*s^\n", pos, "")
				fmt.Fprintln(&buf, "--")

				trace := func(format string, args ...interface{}) {
					msg := fmt.Sprintf(format, args...)
					if curTraceFilter.MatchString(msg) {
						buf.WriteString(msg)
						buf.WriteByte('\n')
					}
				}
				c.(interface {
					TestingSetTraceFn(func(string, ...interface{}))
				}).TestingSetTraceFn(trace)

				var err error
				var hasNext bool
				for hasNext, err = c.Next(ctx); hasNext; hasNext, err = c.Next(ctx) {
					row := c.Values()
					fmt.Fprintf(&buf, "UNEXPECTED ROW: %+v\n", row)
				}
				if err != nil {
					t.Errorf("completion failed: %v", err)
				}
				return buf.String()

			default:
				t.Fatalf("%s: unknown command: %s", td.Pos, td.Cmd)
			}
			return ""
		})
	})
}
