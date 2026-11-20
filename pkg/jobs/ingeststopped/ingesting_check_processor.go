// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package ingeststopped

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/sql/execinfra"
	"github.com/cockroachdb/cockroach/pkg/sql/execinfrapb"
	"github.com/cockroachdb/cockroach/pkg/sql/rowenc"
	"github.com/cockroachdb/cockroach/pkg/sql/rowexec"
	"github.com/cockroachdb/errors"
)

type proc struct {
	execinfra.ProcessorBase
	spec execinfrapb.IngestStoppedSpec
}

func newIngestStoppedProcessor(
	ctx context.Context,
	flowCtx *execinfra.FlowCtx,
	processorID int32,
	spec execinfrapb.IngestStoppedSpec,
	post *execinfrapb.PostProcessSpec,
) (execinfra.Processor, error) {
	p := &proc{spec: spec}
	if err := p.Init(ctx, p, post, nil, flowCtx, processorID, nil /* memMonitor */, execinfra.ProcStateOpts{}); err != nil {
		return nil, err
	}
	return p, nil
}

// Start is part of the RowSource interface.
func (p *proc) Start(ctx context.Context) {
	p.StartInternal(ctx, "ingeststopped.proc")
	if p.FlowCtx.Cfg.JobRegistry.IsIngesting(p.spec.JobID) {
		p.MoveToDraining(errors.Errorf("jobs is still ingesting on node %d", p.FlowCtx.NodeID.SQLInstanceID()))
	} else {
		p.MoveToDraining(nil)
	}
}

// Next is part of the RowSource interface.
func (p *proc) Next() (rowenc.EncDatumRow, *execinfrapb.ProducerMetadata) {
	return nil, p.DrainHelper()
}

func init() {
	rowexec.NewIngestStoppedProcessor = newIngestStoppedProcessor
}
