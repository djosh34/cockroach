// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package spanconfigbounds

import "fmt"

type intI interface{ int32 | int64 }

func formatRange[T intI](start, end T) string {
	if end <= start {
		return fmt.Sprintf("%d", start)
	}
	return fmt.Sprintf("[%d, %d]", start, end)
}

func checkRange[T intI](c *T, start, end T) bool {
	return *c >= start &&
		((end > start && *c <= end) || *c == start)
}

func clampRange[T intI](c *T, start, end T) bool {
	if end < start {
		end = start
	}
	switch {
	case *c < start:
		*c = start
		return true
	case *c > end:
		*c = end
		return true
	default:
		return false
	}
}
