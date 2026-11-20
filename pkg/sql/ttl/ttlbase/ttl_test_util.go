// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package ttlbase

import "fmt"

// GenPKColNames generates column names col0, col1, col2, etc for tests.
func GenPKColNames(numPKCols int) []string {
	names := make([]string, 0, numPKCols)
	for i := 0; i < numPKCols; i++ {
		names = append(names, fmt.Sprintf("col%d", i))
	}
	return names
}
