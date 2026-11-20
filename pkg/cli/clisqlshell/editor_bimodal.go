// Copyright 2022 The Cockroach Authors.
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
	"os"

	"github.com/cockroachdb/errors"
)

// bimodalEditor redirects the input to either a "main" editor
// (outside of COPY mode) or a "copy" editor (inside COPY).
// This is because the main editor may be multi-line and
// thus inadequate for input of COPY data.
type bimodalEditor struct {
	sql  sqlShell
	main editor
	copy editor
}

var _ editor = (*bimodalEditor)(nil)

func (e *bimodalEditor) init(
	win, wout, werr *os.File, sqlS sqlShell, maxHistEntries int, histFile string,
) (cleanupFn func(), err error) {
	e.sql = sqlS
	c1, err := e.main.init(win, wout, werr, sqlS, maxHistEntries, histFile)
	if err != nil {
		return c1, err
	}
	c2, err := e.copy.init(win, wout, werr, sqlS, maxHistEntries, histFile)
	cleanupFn = func() {
		c1()
		c2()
	}
	return cleanupFn, err
}

func (e *bimodalEditor) selected() editor {
	if e.sql.inCopy() {
		return e.copy
	}
	return e.main
}

func (e *bimodalEditor) errInterrupted() error {
	return e.selected().errInterrupted()
}

func (e *bimodalEditor) getOutputStream() *os.File {
	return e.selected().getOutputStream()
}

func (e *bimodalEditor) getLine() (string, error) {
	return e.selected().getLine()
}

func (e *bimodalEditor) addHistory(line string) error {
	return errors.CombineErrors(
		e.copy.addHistory(line),
		e.main.addHistory(line))
}

func (e *bimodalEditor) saveHistory() error {
	return errors.CombineErrors(
		e.copy.saveHistory(),
		e.main.saveHistory())
}

func (e *bimodalEditor) canPrompt() bool {
	return e.main.canPrompt()
}

func (e *bimodalEditor) setPrompt(prompt string) {
	e.copy.setPrompt(prompt)
	e.main.setPrompt(prompt)
}

func (e *bimodalEditor) multilineEdit() bool {
	return e.selected().multilineEdit()
}
