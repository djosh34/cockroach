// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package metric

import (
	"fmt"
	"strings"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/testutils/datapathutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/echotest"
)

// TestHistogramBuckets is used to generate additional prometheus buckets to be
// used with Histogram. Please include obs-inf in the review process of new
// buckets.
func TestHistogramBuckets(t *testing.T) {
	verifyAndPrint := func(t *testing.T, exp []float64, category string) string {
		t.Helper()
		var buf strings.Builder
		for idx, f := range exp {
			if idx == 0 {
				fmt.Fprintf(&buf, "%s", category)
			}
			fmt.Fprintf(&buf, "\n%f", f)
		}
		return buf.String()
	}

	for _, config := range StaticBucketConfigs {
		exp := config.GetBucketsFromBucketConfig()
		buf := verifyAndPrint(t, exp, config.category)

		echotest.Require(t, buf, datapathutils.TestDataPath(t, config.category))
	}

}
