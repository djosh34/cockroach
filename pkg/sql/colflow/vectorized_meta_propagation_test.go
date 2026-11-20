// Copyright 2019 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package colflow_test

import (
	"context"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/col/coldata"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/sql/colexec/colexecargs"
	"github.com/cockroachdb/cockroach/pkg/sql/colexec/colexectestutils"
	"github.com/cockroachdb/cockroach/pkg/sql/colexecop"
	"github.com/cockroachdb/cockroach/pkg/sql/colflow"
	"github.com/cockroachdb/cockroach/pkg/sql/execinfra"
	"github.com/cockroachdb/cockroach/pkg/sql/execinfrapb"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/eval"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/stretchr/testify/require"
)

type testBatchReceiver struct {
	batches  []coldata.Batch
	metadata []*execinfrapb.ProducerMetadata
}

var _ execinfra.BatchReceiver = &testBatchReceiver{}

func (t *testBatchReceiver) ProducerDone() {}

func (t *testBatchReceiver) PushBatch(
	batch coldata.Batch, meta *execinfrapb.ProducerMetadata,
) execinfra.ConsumerStatus {
	status := execinfra.NeedMoreRows
	if batch != nil {
		t.batches = append(t.batches, batch)
	} else if meta != nil {
		t.metadata = append(t.metadata, meta)
	} else {
		status = execinfra.ConsumerClosed
	}
	return status
}

// TestVectorizedMetaPropagation tests whether metadata is correctly propagated
// in the vectorized flows. It creates a colexecop.Operator as well as a
// colexecop.MetadataSource which are hooked up into the BatchFlowCoordinator in
// the same way as in vectorizedFlowCreator.setupFlow.
func TestVectorizedMetaPropagation(t *testing.T) {
	defer leaktest.AfterTest(t)()
	ctx := context.Background()
	st := cluster.MakeTestingClusterSettings()
	evalCtx := eval.MakeTestingEvalContext(st)
	defer evalCtx.Stop(ctx)

	flowCtx := execinfra.FlowCtx{
		EvalCtx: &evalCtx,
		Mon:     evalCtx.TestingMon,
		Cfg:     &execinfra.ServerConfig{Settings: cluster.MakeTestingClusterSettings()},
	}

	nBatches := 10
	typs := types.OneIntCol

	// Prepare the input operator.
	batch := testAllocator.NewMemBatchWithFixedCapacity(typs, 1 /* capacity */)
	batch.SetLength(1)
	source := colexecop.NewRepeatableBatchSource(testAllocator, batch, typs)
	source.ResetBatchesToReturn(nBatches)

	// Setup the metadata source.
	expectedMetadata := []execinfrapb.ProducerMetadata{{RowNum: &execinfrapb.RemoteProducerMetadata_RowNum{LastMsg: true}}}
	drainMetaCbCalled := false
	metadataSource := colexectestutils.CallbackMetadataSource{
		DrainMetaCb: func() []execinfrapb.ProducerMetadata {
			if drainMetaCbCalled {
				return nil
			}
			drainMetaCbCalled = true
			return expectedMetadata
		},
	}

	output := &testBatchReceiver{}
	f := colflow.NewBatchFlowCoordinator(
		&flowCtx,
		0, /* processorID */
		colexecargs.OpWithMetaInfo{
			Root:            source,
			MetadataSources: colexecop.MetadataSources{metadataSource},
		},
		output,
		func() {}, /* cancelFlow */
	)
	f.Run(context.Background())

	// Ensure that the expected number of batches and metadata objects have been
	// pushed into the output.
	require.Equal(t, nBatches, len(output.batches))
	require.Equal(t, len(expectedMetadata), len(output.metadata))
}
