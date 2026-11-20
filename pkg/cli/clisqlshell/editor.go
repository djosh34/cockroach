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

import "os"

// editor is the interface between the shell and a line editor.
type editor interface {
	init(win, wout, werr *os.File, sqlS sqlShell, maxHistEntries int, histFile string) (cleanupFn func(), err error)
	errInterrupted() error
	getOutputStream() *os.File
	getLine() (string, error)
	addHistory(line string) error
	saveHistory() error
	canPrompt() bool
	setPrompt(prompt string)
	multilineEdit() bool
}

type sqlShell interface {
	enableDebug() bool
	inCopy() bool
	runShowCompletions(sql string, offset int) (rows [][]string, err error)
	serverSideParse(sql string) (string, error)
	reflow(
		allText bool, currentText string, targetWidth int,
	) (changed bool, newText string, info string)
}

// getEditor instantiates an editor compatible with the current configuration.
func getEditor(useEditor bool, displayPrompt bool) editor {
	if !useEditor {
		return &bufioReader{displayPrompt: displayPrompt}
	}
	return &bimodalEditor{
		main: &bubblineReader{},
		copy: &bufioReader{displayPrompt: displayPrompt},
	}
}
