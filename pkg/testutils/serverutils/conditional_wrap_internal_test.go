// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package serverutils

import (
	"fmt"
	"strings"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/testutils"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/stretchr/testify/require"
)

func TestBenignNotifyFn(t *testing.T) {
	defer leaktest.AfterTest(t)()

	var logFn func(string, ...interface{})
	var buf strings.Builder
	logFn = func(format string, args ...interface{}) { fmt.Fprintf(&buf, format, args...) }

	defer func(old bool) { reportAllCalls = old }(reportAllCalls)

	testutils.RunTrueAndFalse(t, "reportAllCalls", func(t *testing.T, tb bool) {
		reportAllCalls = tb

		testutils.RunTrueAndFalse(t, "showTip", func(t *testing.T, showTip bool) {
			buf.Reset()
			fn := makeBenignNotifyFn(&logFn, "IN", "ACC", showTip)
			fn("foo")
			fn("foo")
			result := buf.String()
			require.Contains(t, result, "NOTICE: .foo() called via implicit interface IN")
			require.Contains(t, result, "HINT: consider using .ACC().foo() instead")
			if showTip {
				require.Contains(t, result, "TIP:")
			} else {
				require.NotContains(t, result, "TIP:")
			}

			if reportAllCalls {
				require.Equal(t, 2, strings.Count(result, "NOTICE:"))
			} else {
				require.Equal(t, 1, strings.Count(result, "NOTICE:"))
			}
		})
	})
}

func TestSeriousNotifyFn(t *testing.T) {
	defer leaktest.AfterTest(t)()

	var logFn func(string, ...interface{})
	var buf strings.Builder
	logFn = func(format string, args ...interface{}) { fmt.Fprintf(&buf, format, args...) }

	defer func(old bool) { reportAllCalls = old }(reportAllCalls)

	testutils.RunTrueAndFalse(t, "reportAllCalls", func(t *testing.T, tb bool) {
		reportAllCalls = tb

		buf.Reset()
		fn := makeSeriousNotifyFn(&logFn, "IN", "ACC1", "ACC2")
		fn("foo")
		fn("foo")
		result := buf.String()
		require.Contains(t, result, "WARNING: risky use of implicit IN via .foo()")
		require.Contains(t, result, "HINT: clarify intent using .ACC1().foo() or .ACC2().foo() instead")
		require.Contains(t, result, "TIP:")

		if reportAllCalls {
			require.Equal(t, 2, strings.Count(result, "WARNING:"))
		} else {
			require.Equal(t, 1, strings.Count(result, "WARNING:"))
		}
	})
}
