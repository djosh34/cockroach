// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package clisqlexec

import (
	"fmt"
	"html"
	"io"
	"strings"

	"github.com/cockroachdb/cockroach/pkg/util"
)

type htmlReporter struct {
	nCols int

	escape   bool
	rowStats bool
}

func (p *htmlReporter) describe(w io.Writer, cols []string) error {
	p.nCols = len(cols)
	fmt.Fprint(w, "<table>\n<thead><tr>")
	if p.rowStats {
		fmt.Fprint(w, "<th>row</th>")
	}
	for _, col := range cols {
		if p.escape {
			col = html.EscapeString(col)
		}
		fmt.Fprintf(w, "<th>%s</th>", strings.Replace(col, "\n", "<br/>", -1))
	}
	fmt.Fprintln(w, "</tr></thead>")
	return nil
}

func (p *htmlReporter) beforeFirstRow(w io.Writer, _ RowStrIter) error {
	fmt.Fprintln(w, "<tbody>")
	return nil
}

func (p *htmlReporter) iter(w, _ io.Writer, rowIdx int, row []string) error {
	fmt.Fprint(w, "<tr>")
	if p.rowStats {
		fmt.Fprintf(w, "<td>%d</td>", rowIdx+1)
	}
	for _, r := range row {
		if p.escape {
			r = html.EscapeString(r)
		}
		fmt.Fprintf(w, "<td>%s</td>", strings.Replace(r, "\n", "<br/>", -1))
	}
	fmt.Fprintln(w, "</tr>")
	return nil
}

func (p *htmlReporter) doneNoRows(w io.Writer) error {
	fmt.Fprintln(w, "</table>")
	return nil
}

func (p *htmlReporter) doneRows(w io.Writer, nRows int) error {
	fmt.Fprintln(w, "</tbody>")
	if p.rowStats {
		fmt.Fprintf(w, "<tfoot><tr><td colspan=%d>%d row%s</td></tr></tfoot>",
			p.nCols+1, nRows, util.Pluralize(int64(nRows)))
	}
	fmt.Fprintln(w, "</table>")
	return nil
}
