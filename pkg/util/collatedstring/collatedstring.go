// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package collatedstring

import "golang.org/x/text/collate"

// DefaultCollationTag is the "default" collation for strings.
const DefaultCollationTag = "default"

// CCollationTag is the "C" collation for strings. At the time of writing
// this, it behaves the same os "default" since LC_COLLATE cannot be modified.
const CCollationTag = "C"

// PosixCollationTag is the "POSIX" collation for strings. At the time of
// writing this, it behaves the same os "default" since LC_COLLATE cannot be
// modified.
const PosixCollationTag = "POSIX"

var supportedTagNames []string

func IsDefaultEquivalentCollation(s string) bool {
	return s == DefaultCollationTag || s == CCollationTag || s == PosixCollationTag
}

// Supported returns a list of all the collation names that are supported.
func Supported() []string {
	return supportedTagNames
}

func init() {
	if collate.CLDRVersion != "23" {
		panic("This binary was built with an incompatible version of golang.org/x/text. " +
			"See https://github.com/cockroachdb/cockroach/issues/63738 for details")
	}

	supportedTagNames = []string{
		DefaultCollationTag,
		CCollationTag,
		PosixCollationTag,
	}
	for _, t := range collate.Supported() {
		supportedTagNames = append(supportedTagNames, t.String())
	}
}
