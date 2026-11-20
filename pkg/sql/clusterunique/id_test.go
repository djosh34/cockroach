// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package clusterunique

import (
	"testing"

	"github.com/cockroachdb/cockroach/pkg/util/uint128"
	"github.com/stretchr/testify/require"
)

func TestCompare(t *testing.T) {
	testCases := []struct {
		name               string
		aHi, aLo, bHi, bLo uint64
		expected           int
	}{
		{
			name:     "high less",
			bHi:      1,
			expected: -1,
		},
		{
			name:     "high equal, low less",
			bLo:      1,
			expected: -1,
		},
		{
			name:     "high equal, low equal",
			expected: 0,
		},
		{
			name:     "high equal, low greater",
			aLo:      1,
			expected: 1,
		},
		{
			name:     "high greater",
			aHi:      1,
			expected: 1,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			a := ID{Uint128: uint128.FromInts(tc.aHi, tc.aLo)}
			b := ID{Uint128: uint128.FromInts(tc.bHi, tc.bLo)}
			require.Equal(t, tc.expected, a.Compare(b))
		})
	}
}
