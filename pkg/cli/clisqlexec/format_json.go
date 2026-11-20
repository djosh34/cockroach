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
	"encoding/json"
	"io"
)

type jsonReporter struct {
	start, end, middle []byte
	currentPrefix      []byte
	cols               []string
	rowMap             map[string]string
}

func (n *jsonReporter) describe(w io.Writer, cols []string) error {
	n.cols = cols
	n.rowMap = make(map[string]string, len(cols))
	for _, col := range cols {
		n.rowMap[col] = ""
	}
	_, err := w.Write(n.start)
	return err
}

func (n *jsonReporter) beforeFirstRow(w io.Writer, allRows RowStrIter) error {
	return nil
}

func (n *jsonReporter) iter(w, ew io.Writer, rowIdx int, row []string) error {
	for i := range row {
		n.rowMap[n.cols[i]] = row[i]
	}
	out, err := json.Marshal(n.rowMap)
	if err != nil {
		return err
	}
	if _, err := w.Write(n.currentPrefix); err != nil {
		return err
	}
	n.currentPrefix = n.middle
	if _, err := w.Write(out); err != nil {
		return err
	}
	return nil
}

func (n *jsonReporter) doneRows(w io.Writer, seenRows int) error {
	_, err := w.Write(n.end)
	return err
}

func (n *jsonReporter) doneNoRows(w io.Writer) error {
	_, err := w.Write(n.end)
	return err
}

func makeJSONReporter(format TableDisplayFormat) *jsonReporter {
	r := &jsonReporter{
		start:         []byte("["),
		end:           []byte("\n]\n"),
		middle:        []byte(",\n  "),
		currentPrefix: []byte("\n  "),
	}
	if format == TableDisplayNDJSON {
		r.start = nil
		r.currentPrefix = nil
		r.end = []byte("\n")
		r.middle = []byte("\n")
	}
	return r
}
