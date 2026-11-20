// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package rel

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestOrdinalSetCapacity ensures that ordinalSet operations
func TestOrdinalSetCapacity(t *testing.T) {
	var os ordinalSet
	for o := ordinal(0); o <= ordinalSetMaxOrdinal; o++ {
		added := os.add(o)
		require.NotEqual(t, os, added)
		added = os
	}

	// Adding a value which is out-of-range should be a no-op.
	require.Equal(t, os, os.add(ordinalSetMaxOrdinal+1))
}
