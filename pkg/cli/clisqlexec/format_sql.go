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
	"bytes"
	"fmt"
	"io"

	"github.com/cockroachdb/cockroach/pkg/sql/lexbase"
	"github.com/cockroachdb/cockroach/pkg/util"
)

type sqlReporter struct {
	noColumns bool
}

func (p *sqlReporter) describe(w io.Writer, cols []string) error {
	fmt.Fprint(w, "CREATE TABLE results (\n")
	for i, col := range cols {
		var colName bytes.Buffer
		lexbase.EncodeRestrictedSQLIdent(&colName, col, lexbase.EncNoFlags)
		fmt.Fprintf(w, "  %s STRING", colName.String())
		if i < len(cols)-1 {
			fmt.Fprint(w, ",")
		}
		fmt.Fprint(w, "\n")
	}
	fmt.Fprint(w, ");\n\n")
	p.noColumns = (len(cols) == 0)
	return nil
}

func (p *sqlReporter) iter(w, _ io.Writer, _ int, row []string) error {
	if p.noColumns {
		fmt.Fprintln(w, "INSERT INTO results(rowid) VALUES (DEFAULT);")
		return nil
	}

	fmt.Fprint(w, "INSERT INTO results VALUES (")
	for i, r := range row {
		var buf bytes.Buffer
		lexbase.EncodeSQLStringWithFlags(&buf, r, lexbase.EncNoFlags)
		fmt.Fprint(w, buf.String())
		if i < len(row)-1 {
			fmt.Fprint(w, ", ")
		}
	}
	fmt.Fprint(w, ");\n")
	return nil
}

func (p *sqlReporter) beforeFirstRow(_ io.Writer, _ RowStrIter) error { return nil }
func (p *sqlReporter) doneNoRows(_ io.Writer) error                   { return nil }
func (p *sqlReporter) doneRows(w io.Writer, seenRows int) error {
	fmt.Fprintf(w, "-- %d row%s\n", seenRows, util.Pluralize(int64(seenRows)))
	return nil
}
