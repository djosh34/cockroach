// Copyright 2017 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package testutils

import (
	"fmt"
	"testing"
)

// RunTrueAndFalse calls the provided function in a subtest, first with the
// boolean argument set to false and next with the boolean argument set to true.
func RunTrueAndFalse(t *testing.T, name string, fn func(t *testing.T, b bool)) {
	t.Helper()
	RunValues(t, name, []bool{false, true}, fn)
}

// RunValues calls the provided function in a subtest for each of the
// provided values.
func RunValues[T any](t *testing.T, name string, values []T, fn func(*testing.T, T)) {
	t.Helper()
	for _, v := range values {
		t.Run(fmt.Sprintf("%s=%v", name, v), func(t *testing.T) {
			fn(t, v)
		})
	}
}
