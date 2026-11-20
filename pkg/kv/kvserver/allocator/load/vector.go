// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package load

import (
	"fmt"
	"strings"
)

// Vector is a static container which implements the Load interface.
type Vector [nDimensions]float64

var _ Load = Vector{}

// Dim returns the value of the Dimension given.
func (s Vector) Dim(dim Dimension) float64 {
	if int(dim) > len(s) || dim < 0 {
		panic(fmt.Sprintf("Unknown load dimension access, %d", dim))
	}
	return s[dim]
}

// String returns a string representation of Load.
func (s Vector) String() string {
	var buf strings.Builder

	fmt.Fprint(&buf, "(")
	for i, val := range s {
		if i > 0 {
			fmt.Fprint(&buf, " ")
		}
		dim := Dimension(i)
		fmt.Fprintf(&buf, "%s=%s", dim.String(), dim.Format(val))
	}
	fmt.Fprint(&buf, ")")
	return buf.String()
}
