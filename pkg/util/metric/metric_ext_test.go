// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package metric_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/cockroachdb/cockroach/pkg/testutils/datapathutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/echotest"
	"github.com/cockroachdb/cockroach/pkg/util/metric"
	"github.com/stretchr/testify/require"
)

func TestHistogramPrometheus(t *testing.T) {
	// Regression test against https://github.com/cockroachdb/cockroach/pull/88331.
	// The output includes buckets for which the upper bound equals the previous
	// bucket's upper bound.
	h := metric.NewHistogram(metric.HistogramOptions{
		Mode:     metric.HistogramModePrometheus,
		Metadata: metric.Metadata{},
		Duration: time.Second,
		Buckets:  []float64{1, 2, 3, 4, 5, 6, 10, 20, 30},
	})
	h.RecordValue(1)
	h.RecordValue(5)
	h.RecordValue(5)
	h.RecordValue(10)
	hist := h.ToPrometheusMetric().Histogram
	// The creation time differs on every run.
	hist.CreatedTimestamp = nil
	act, err := json.MarshalIndent(hist, "", "  ")
	require.NoError(t, err)
	echotest.Require(t, string(act), datapathutils.TestDataPath(t, "histogram.txt"))
}
