// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package split

import (
	"fmt"
	"time"

	"github.com/cockroachdb/cockroach/pkg/util/humanizeutil"
)

// SplitObjective is a type that specifies a load based splitting objective.
type SplitObjective int

const (
	// SplitQPS will track and split QPS (queries-per-second) over a range.
	SplitQPS SplitObjective = iota
	// SplitCPU will track and split CPU (cpu-per-second) over a range.
	SplitCPU
)

// String returns a human readable string representation of the dimension.
func (d SplitObjective) String() string {
	switch d {
	case SplitQPS:
		return "qps"
	case SplitCPU:
		return "cpu"
	default:
		panic(fmt.Sprintf("cannot name: unknown objective with ordinal %d", d))
	}
}

// Format returns a formatted string for a value.
func (d SplitObjective) Format(value float64) string {
	switch d {
	case SplitQPS:
		return fmt.Sprintf("%.1f", value)
	case SplitCPU:
		return string(humanizeutil.Duration(time.Duration(int64(value))))
	default:
		panic(fmt.Sprintf("cannot format value: unknown objective with ordinal %d", d))
	}
}
