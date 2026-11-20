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
	"io"
	"time"

	"github.com/cockroachdb/cockroach/pkg/util/encoding/csv"
	"github.com/cockroachdb/cockroach/pkg/util/syncutil"
)

type csvReporter struct {
	mu struct {
		syncutil.Mutex
		csvWriter *csv.Writer
	}
	stop chan struct{}
}

// csvFlushInterval is the maximum time between flushes of the
// buffered CSV/TSV data.
const csvFlushInterval = 5 * time.Second

func makeCSVReporter(w io.Writer, format TableDisplayFormat) (*csvReporter, func()) {
	r := &csvReporter{}
	r.mu.csvWriter = csv.NewWriter(w)
	if format == TableDisplayTSV {
		r.mu.csvWriter.Comma = '\t'
	}

	// Set up a flush daemon. This is useful when e.g. visualizing data
	// from change feeds.
	r.stop = make(chan struct{}, 1)
	go func() {
		ticker := time.NewTicker(csvFlushInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				r.mu.Lock()
				r.mu.csvWriter.Flush()
				r.mu.Unlock()
			case <-r.stop:
				return
			}
		}
	}()
	cleanup := func() {
		close(r.stop)
	}
	return r, cleanup
}

func (p *csvReporter) describe(w io.Writer, cols []string) error {
	p.mu.Lock()
	if len(cols) == 0 {
		_ = p.mu.csvWriter.Write([]string{"# no columns"})
	} else {
		_ = p.mu.csvWriter.Write(cols)
	}
	p.mu.Unlock()
	return nil
}

func (p *csvReporter) iter(_, _ io.Writer, _ int, row []string) error {
	p.mu.Lock()
	if len(row) == 0 {
		_ = p.mu.csvWriter.Write([]string{"# empty"})
	} else {
		_ = p.mu.csvWriter.Write(row)
	}
	p.mu.Unlock()
	return nil
}

func (p *csvReporter) beforeFirstRow(_ io.Writer, _ RowStrIter) error { return nil }
func (p *csvReporter) doneNoRows(_ io.Writer) error                   { return nil }

func (p *csvReporter) doneRows(w io.Writer, seenRows int) error {
	p.mu.Lock()
	p.mu.csvWriter.Flush()
	p.mu.Unlock()
	return nil
}
