// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package allstacks

import "runtime"

// Get returns all stacks, except if that takes more than 512mb of memory, in
// which case it returns only 512mb worth of stacks (clipping the last stack
// if necessary).
func Get() []byte {
	return GetWithBuf(nil)
}

// GetWithBuf is like Get, but tries to use the provided slice first, allocating
// a new, larger, slice only if necessary.
func GetWithBuf(buf []byte) []byte {
	buf = buf[:cap(buf)]
	// We don't know how big the traces are, so grow a few times if they don't
	// fit. Start large, though.
	for n := 1 << 20; /* 1mb */ n <= (1 << 29); /* 512mb */ n *= 2 {
		if len(buf) < n {
			buf = make([]byte, n)
		}
		nbytes := runtime.Stack(buf, true /* all */)
		if nbytes < len(buf) {
			return buf[:nbytes]
		}
	}
	return buf // returning full 512MB slice
}
