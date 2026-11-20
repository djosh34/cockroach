// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package tochar

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormatCache(t *testing.T) {
	c := NewFormatCache(1)
	hh24 := parseFormat("HH24")

	// Cache is correctly populated.
	result := c.lookup("HH24")
	require.Equal(t, hh24, result)
	rInt, ok := c.mu.cache.Get("HH24")
	require.True(t, ok)
	require.Equal(t, hh24, rInt.([]formatNode))

	// Cache does not cache large format strings.
	longStr := strings.Repeat(",", maxCacheKeySize+1)
	longFmt := parseFormat(longStr)
	// result should still be the same.
	result = c.lookup(longStr)
	require.Equal(t, longFmt, result)
	// But the entry should not be in the cache.
	_, ok = c.mu.cache.Get(longStr)
	require.False(t, ok)
	_, ok = c.mu.cache.Get("HH24")
	require.True(t, ok)
}

// TestFormatCacheLookupThreadSafe is non-deterministic. Flakes indicate that
// FormatCache.lookup is not thread safe.
// See https://github.com/cockroachdb/cockroach/issues/95424
func TestFormatCacheLookupThreadSafe(t *testing.T) {
	formats := []string{"HH12", "HH24", "MI"}
	c := NewFormatCache(len(formats) - 1)
	for i := 0; i < 100_000; i++ {
		go func(i int) {
			format := formats[i%len(formats)]
			c.lookup(format)
		}(i)
	}
}
