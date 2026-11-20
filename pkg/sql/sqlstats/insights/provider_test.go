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
	"testing"

	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/sql/clusterunique"
	"github.com/stretchr/testify/require"
)

func TestProvider_WriterIgnoresInternalExecutorObservations(t *testing.T) {
	settings := cluster.MakeTestingClusterSettings()
	store := newStore(settings)
	ingester := newConcurrentBufferIngester(newRegistry(settings, &fakeDetector{stubEnabled: true}, store))
	provider := &defaultProvider{store: store, ingester: ingester}
	writer := provider.Writer(true /* internal */)
	writer.ObserveStatement(clusterunique.ID{}, &Statement{})
	writer.ObserveTransaction(clusterunique.ID{}, &Transaction{})
	require.Equal(t, event{}, ingester.guard.eventBuffer[0])
}
