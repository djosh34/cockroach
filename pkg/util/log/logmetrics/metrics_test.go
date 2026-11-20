// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package logmetrics

import (
	"testing"

	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/metric"
	"github.com/stretchr/testify/require"
)

func TestIncrementCounter(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	l := newLogMetricsRegistry()

	metrics := map[log.Metric]*metric.Counter{
		log.FluentSinkConnectionAttempt: l.metricsStruct.FluentSinkConnAttempts,
		log.FluentSinkConnectionError:   l.metricsStruct.FluentSinkConnErrors,
		log.FluentSinkWriteAttempt:      l.metricsStruct.FluentSinkWriteAttempts,
		log.FluentSinkWriteError:        l.metricsStruct.FluentSinkWriteErrors,
		log.BufferedSinkMessagesDropped: l.metricsStruct.BufferedSinkMessagesDropped,
		log.LogMessageCount:             l.metricsStruct.LogMessageCount,
	}
	func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		for _, m := range metrics {
			require.Zero(t, m.Count())
		}
	}()
	for m := range metrics {
		l.IncrementCounter(m, 1)
		l.IncrementCounter(m, 2)
	}
	func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		for _, m := range metrics {
			require.Equal(t, int64(3), m.Count())
		}
	}()
}

func TestNewRegistry(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	t.Run("panics when logMetricsReg is nil", func(t *testing.T) {
		logMetricsReg = nil
		require.PanicsWithErrorf(t,
			"LogMetricsRegistry was not initialized",
			func() {
				_ = NewRegistry()
			}, "expected NewRegistry() to panic with nil logMetricsReg package-level var")
	})
}

type fakeLogMetrics struct{}

func (*fakeLogMetrics) IncrementCounter(_ log.Metric, _ int64) {}

var _ log.LogMetrics = (*fakeLogMetrics)(nil)
