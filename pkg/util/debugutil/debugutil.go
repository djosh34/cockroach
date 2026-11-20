// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package debugutil

import (
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/elastic/gosigar"
)

// IsLaunchedByDebugger returns true in cases where the delve debugger
// was used to launch the cockroach process.
func IsLaunchedByDebugger() bool {
	return isLaunchedByDebugger.Load()
}

var isLaunchedByDebugger atomic.Bool

func init() {
	isLaunchedByDebugger.Store(func(maybeDelvePID int) bool {
		// We loop in case there were intermediary processes like the gopls
		// language server.
		for maybeDelvePID != 0 {
			var exe gosigar.ProcExe
			if err := exe.Get(maybeDelvePID); err != nil {
				break
			}
			switch filepath.Base(exe.Name) {
			case "dlv":
				return true
			}
			var state gosigar.ProcState
			if err := state.Get(maybeDelvePID); err != nil {
				break
			}
			maybeDelvePID = state.Ppid
		}
		return false
	}(os.Getppid()))
}
