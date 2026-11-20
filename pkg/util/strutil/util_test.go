// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.
package strutil

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAppendInt(t *testing.T) {
	for _, tc := range []struct {
		n     int
		width int
		fmt   string
	}{
		{0, 3, "%03d"},
		{5, 2, "%02d"},
		{10, 1, "%01d"},
		{-11, 4, "%04d"},
		{1234, 6, "%06d"},
		{-321, 2, "%02d"},
	} {
		require.Equal(t, fmt.Sprintf(tc.fmt, tc.n), string(AppendInt(nil, tc.n, tc.width)))
	}
}
