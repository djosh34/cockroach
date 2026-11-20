// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package insights

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/sql/clusterunique"
	"github.com/cockroachdb/cockroach/pkg/util/stop"
)

type defaultProvider struct {
	store           *lockingStore
	ingester        *concurrentBufferIngester
	anomalyDetector *anomalyDetector
}

var _ Provider = &defaultProvider{}

func (p *defaultProvider) Start(ctx context.Context, stopper *stop.Stopper) {
	p.ingester.Start(ctx, stopper)
}

func (p *defaultProvider) Writer(internal bool) Writer {
	// We ignore statements and transactions run by the internal executor.
	if internal {
		return nullWriterInstance
	}
	return p.ingester
}

func (p *defaultProvider) Reader() Reader {
	return p.store
}

func (p *defaultProvider) LatencyInformation() LatencyInformation {
	return p.anomalyDetector
}

type nullWriter struct{}

func (n *nullWriter) ObserveStatement(_ clusterunique.ID, _ *Statement) {
}

func (n *nullWriter) ObserveTransaction(_ clusterunique.ID, _ *Transaction) {
}

var nullWriterInstance Writer = &nullWriter{}
