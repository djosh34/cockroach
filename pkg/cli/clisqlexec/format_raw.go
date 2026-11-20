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
	"io"

	"github.com/cockroachdb/cockroach/pkg/util"
)

type rawReporter struct{}

func (p *rawReporter) describe(w io.Writer, cols []string) error {
	fmt.Fprintf(w, "# %d column%s\n", len(cols),
		util.Pluralize(int64(len(cols))))
	return nil
}

func (p *rawReporter) iter(w, _ io.Writer, rowIdx int, row []string) error {
	fmt.Fprintf(w, "# row %d\n", rowIdx+1)
	for _, r := range row {
		fmt.Fprintf(w, "## %d\n%s\n", len(r), r)
	}
	return nil
}

func (p *rawReporter) beforeFirstRow(_ io.Writer, _ RowStrIter) error { return nil }
func (p *rawReporter) doneNoRows(_ io.Writer) error                   { return nil }

func (p *rawReporter) doneRows(w io.Writer, nRows int) error {
	fmt.Fprintf(w, "# %d row%s\n", nRows, util.Pluralize(int64(nRows)))
	return nil
}
