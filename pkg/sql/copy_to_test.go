// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package sql

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/stretchr/testify/require"
)

func TestEncodeCopy(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	testCases := []struct {
		in        string
		expected  string
		delimiter byte
	}{
		{in: `ab|c`, expected: `ab|c`, delimiter: '\t'},
		{in: `ab|c`, expected: `ab\|c`, delimiter: '|'},
		{in: `ab|c` + string("\t\r\n\t\b\t\f\\") + "|d", expected: `ab\|c\t\r\n\t\b\t\f\\\|d`, delimiter: '|'},
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s, delimiter %c", tc.in, tc.delimiter), func(t *testing.T) {
			var b bytes.Buffer
			require.NoError(t, EncodeCopy(&b, []byte(tc.in), tc.delimiter))
			require.Equal(t, tc.expected, b.String())

			// Check decode is the same.
			require.Equal(t, tc.in, DecodeCopy(tc.expected))
		})
	}
}
