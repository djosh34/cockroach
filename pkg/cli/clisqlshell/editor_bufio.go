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
	"bufio"
	"fmt"
	"io"
	"os"

	"github.com/cockroachdb/errors"
)

// bufioReader implements the editor interface.
type bufioReader struct {
	wout          *os.File
	displayPrompt bool
	prompt        string
	buf           *bufio.Reader
}

var _ editor = (*bufioReader)(nil)

func (b *bufioReader) init(
	win, wout, werr *os.File, _ sqlShell, maxHistEntries int, histFile string,
) (cleanupFn func(), err error) {
	b.wout = wout
	b.buf = bufio.NewReader(win)
	return func() {}, nil
}

var errBufioInterrupted = errors.New("never happens")

func (b *bufioReader) errInterrupted() error {
	return errBufioInterrupted
}

func (b *bufioReader) getOutputStream() *os.File {
	return b.wout
}

func (b *bufioReader) addHistory(line string) error {
	return nil
}

func (b *bufioReader) saveHistory() error {
	return nil
}

func (b *bufioReader) canPrompt() bool {
	return b.displayPrompt
}

func (b *bufioReader) setPrompt(prompt string) {
	if b.displayPrompt {
		b.prompt = prompt
	}
}

func (b *bufioReader) multilineEdit() bool {
	return false
}

func (b *bufioReader) getLine() (string, error) {
	fmt.Fprint(b.wout, b.prompt)
	l, err := b.buf.ReadString('\n')
	// bufio.ReadString() differs from readline.Readline in the handling of
	// EOF. Readline only returns EOF when there is nothing left to read and
	// there is no partial line while bufio.ReadString() returns EOF when the
	// end of input has been reached but will return the non-empty partial line
	// as well. We work around this by converting the bufioReader behavior to match
	// the Readline behavior.
	if err == io.EOF && len(l) != 0 {
		err = nil
	} else if err == nil {
		// From the bufio.ReadString docs: ReadString returns err != nil if and
		// only if the returned data does not end in delim. To match the behavior
		// of readline.Readline, we strip off the trailing delimiter.
		l = l[:len(l)-1]
	}
	return l, err
}
