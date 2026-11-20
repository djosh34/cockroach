// Copyright 2021 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package testutils

// TestFataler is a slimmed down version of testing.TB for use in helper functions
// by testing contexts which do not come from the stdlib testing package.
type TestFataler interface {
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
	Helper()
}

// TestErrorer is like Fataler but it only needs a Error method.
type TestErrorer interface {
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Helper()
}

// TestFatalerLogger is like Fataler but it also needs a Log method.
type TestFatalerLogger interface {
	TestFataler
	Logf(format string, args ...interface{})
}

// TestNamedFatalerLogger is like Fataler but it also needs a Name method.
type TestNamedFatalerLogger interface {
	TestFatalerLogger
	Name() string
}
