// Copyright 2022 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package bulk_test

import (
	"context"
	"testing"
	"time"

	"github.com/cockroachdb/cockroach/pkg/kv/bulk/bulkpb"
	"github.com/cockroachdb/cockroach/pkg/util/bulk"
	"github.com/cockroachdb/cockroach/pkg/util/tracing"
	"github.com/cockroachdb/cockroach/pkg/util/tracing/tracingpb"
	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"
)

func TestAggregator(t *testing.T) {
	tr := tracing.NewTracer()
	ctx := context.Background()
	ctx, root := tr.StartSpanCtx(ctx, "root", tracing.WithRecording(tracingpb.RecordingVerbose))
	defer root.Finish()

	agg := bulk.TracingAggregatorForContext(ctx)
	ctx, withListener := tr.StartSpanCtx(ctx, "withListener",
		tracing.WithEventListeners(agg), tracing.WithParent(root))
	defer withListener.Finish()

	child := tr.StartSpan("child", tracing.WithParent(withListener))
	defer child.Finish()
	child.RecordStructured(&bulkpb.IngestionPerformanceStats{
		Batches:         10,
		LogicalDataSize: 10,
		Duration:        time.Minute,
	})

	_, childChild := tracing.ChildSpan(ctx, "childChild")
	defer childChild.Finish()
	childChild.RecordStructured(&bulkpb.IngestionPerformanceStats{
		Batches:         20,
		LogicalDataSize: 20,
		Duration:        time.Minute,
	})

	remoteChild := tr.StartSpan("remoteChild", tracing.WithRemoteParentFromSpanMeta(childChild.Meta()))
	remoteChild.RecordStructured(&bulkpb.IngestionPerformanceStats{
		Batches:         30,
		LogicalDataSize: 30,
		Duration:        time.Minute,
	})

	// We only expect to see the aggregated stats from the local children since we
	// have not imported the remote children's Recording.
	agg.ForEachAggregatedEvent(func(name string, event bulk.TracingAggregatorEvent) {
		require.Equal(t, name, proto.MessageName(&bulkpb.IngestionPerformanceStats{}))
		var es *bulkpb.IngestionPerformanceStats
		var ok bool
		if es, ok = event.(*bulkpb.IngestionPerformanceStats); !ok {
			t.Fatal("failed to cast event to expected type")
		}
		require.Equal(t, int64(30), es.Batches)
		require.Equal(t, int64(30), es.LogicalDataSize)
		require.Equal(t, 2*time.Minute, es.Duration)
	})

	// Import the remote recording into its parent.
	rec := remoteChild.FinishAndGetConfiguredRecording()
	childChild.ImportRemoteRecording(rec)

	// Now, we expect the stats from the remote child to show up in the
	// aggregator.
	agg.ForEachAggregatedEvent(func(name string, event bulk.TracingAggregatorEvent) {
		require.Equal(t, name, proto.MessageName(&bulkpb.IngestionPerformanceStats{}))
		var es *bulkpb.IngestionPerformanceStats
		var ok bool
		if es, ok = event.(*bulkpb.IngestionPerformanceStats); !ok {
			t.Fatal("failed to cast event to expected type")
		}
		require.Equal(t, int64(60), es.Batches)
		require.Equal(t, int64(60), es.LogicalDataSize)
		require.Equal(t, 3*time.Minute, es.Duration)
	})
}
