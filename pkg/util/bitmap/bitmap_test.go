// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package bitmap

import (
	"testing"

	"github.com/cockroachdb/cockroach/pkg/util/randutil"
	"github.com/stretchr/testify/require"
)

func TestBitmap(t *testing.T) {
	rng, _ := randutil.NewTestRand()
	n := rng.Intn(1000) + 1

	bm := NewBitmap(n)
	naive := make([]bool, n)
	for op := 0; op < 1000; op++ {
		i := rng.Intn(n)
		bm.Set(i)
		naive[i] = true
		for j := 0; j < n; j++ {
			require.Equal(t, naive[j], bm.IsSet(j))
		}
	}
}
