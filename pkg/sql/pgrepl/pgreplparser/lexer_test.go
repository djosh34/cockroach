// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package pgreplparser

import (
	"fmt"
	"strings"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/sql/pgrepl/lsn"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/testutils/datapathutils"
	"github.com/cockroachdb/datadriven"
)

func TestLexer(t *testing.T) {
	datadriven.Walk(t, datapathutils.TestDataPath(t, "lexer"), func(t *testing.T, path string) {
		datadriven.RunTest(t, path, func(t *testing.T, td *datadriven.TestData) string {
			switch td.Cmd {
			case "lex":
				var sb strings.Builder
				var st pgreplSymType
				s := newLexer(td.Input)
				var started bool
				for !s.done() {
					if started {
						sb.WriteRune('\n')
					}
					started = true
					s.lex(&st)
					if s.lastError != nil {
						sb.WriteString(fmt.Sprintf("ERROR: %s", s.lastError.Error()))
						break
					}
					switch st.id {
					case IDENT:
						sb.WriteString("IDENT")
					case SCONST:
						sb.WriteString("SCONST")
					case UCONST:
						sb.WriteString("UCONST")
					case RECPTR:
						sb.WriteString("LSN")
					default:
						sb.WriteString(fmt.Sprintf("id:%d", st.id))
					}
					sb.WriteString(fmt.Sprintf(" str:%s", st.str))
					if st.union.val != nil {
						switch v := st.union.val.(type) {
						case lsn.LSN:
							sb.WriteString(fmt.Sprintf(" lsn:%s", v))
						case *tree.NumVal:
							sb.WriteString(fmt.Sprintf(" num:%s", v))
						default:
							t.Errorf("unknown union type: %T", v)
						}
					}
				}
				return sb.String()
			default:
				t.Errorf("unknown command %s", td.Cmd)
			}
			return ""
		})
	})
}
