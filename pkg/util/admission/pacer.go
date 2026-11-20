// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package admission

import (
	"context"
	"time"
)

// Pacer is used in tight loops (CPU-bound) for non-premptible elastic work.
// Callers are expected to invoke Pace() every loop iteration and Close() once
// done. Internally this type integrates with elastic CPU work queue, acquiring
// tokens for the CPU work being done, and blocking if tokens are unavailable.
// This allows for a form of cooperative scheduling with elastic CPU granters.
type Pacer struct {
	unit time.Duration
	wi   WorkInfo
	wq   *ElasticCPUWorkQueue

	cur *ElasticCPUWorkHandle
}

// Pace is part of the Pacer interface.
func (p *Pacer) Pace(ctx context.Context) error {
	if p == nil {
		return nil
	}

	if overLimit, _ := p.cur.OverLimit(); overLimit {
		p.wq.AdmittedWorkDone(p.cur)
		p.cur = nil
	}

	if p.cur == nil {
		handle, err := p.wq.Admit(ctx, p.unit, p.wi)
		if err != nil {
			return err
		}
		p.cur = handle
	}
	return nil
}

// Close is part of the Pacer interface.
func (p *Pacer) Close() {
	if p == nil || p.cur == nil {
		return
	}

	p.wq.AdmittedWorkDone(p.cur)
	p.cur = nil
}
