// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package metrics_test

import (
	"context"
	"testing"
	"time"

	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/asim"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/asim/config"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/asim/metrics"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/asim/scheduled"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/asim/state"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/asim/workload"
	"github.com/stretchr/testify/require"
)

type mockListener struct {
	history [][]metrics.StoreMetrics
}

func (ml *mockListener) Listen(ctx context.Context, sms []metrics.StoreMetrics) {
	ml.history = append(ml.history, sms)
}

// TestTracker asserts that the Tracker calls Listen on each registered
// listener with identical arguments.
func TestTracker(t *testing.T) {
	ctx := context.Background()
	settings := config.DefaultSimulationSettings()
	duration := 200 * time.Second
	rwg := []workload.Generator{
		workload.TestCreateWorkloadGenerator(settings.Seed, settings.StartTime, 10, 10000),
	}
	s := state.LoadConfig(state.ComplexConfig, state.SingleRangeConfig, settings)
	l1 := &mockListener{history: [][]metrics.StoreMetrics{}}
	l2 := &mockListener{history: [][]metrics.StoreMetrics{}}
	tracker := metrics.NewTracker(testingMetricsInterval, l1, l2)

	sim := asim.NewSimulator(duration, rwg, s, settings, tracker, scheduled.NewExecutorWithNoEvents())
	sim.RunSim(ctx)

	require.Equal(t, l1.history, l2.history)
}
