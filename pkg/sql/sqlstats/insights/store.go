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
	"sync/atomic"

	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/util/cache"
	"github.com/cockroachdb/cockroach/pkg/util/syncutil"
)

type lockingStore struct {
	// stmtCount keeps track of the number of statement insights
	// that have been observed in the underlying cache.
	stmtCount atomic.Int64

	mu struct {
		syncutil.RWMutex
		insights *cache.UnorderedCache
	}
}

func (s *lockingStore) AddInsight(insight *Insight) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stmtCount.Add(int64(len(insight.Statements)))
	s.mu.insights.Add(insight.Transaction.ID, insight)
}

func (s *lockingStore) IterateInsights(
	ctx context.Context, visitor func(context.Context, *Insight),
) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	s.mu.insights.Do(func(e *cache.Entry) {
		visitor(ctx, e.Value.(*Insight))
	})
}

var _ Reader = &lockingStore{}
var _ sink = &lockingStore{}

func newStore(st *cluster.Settings) *lockingStore {
	s := &lockingStore{}
	config := cache.Config{
		Policy: cache.CacheFIFO,
		ShouldEvict: func(size int, key, value interface{}) bool {
			return s.stmtCount.Load() > ExecutionInsightsCapacity.Get(&st.SV)
		},
		OnEvicted: func(_, value interface{}) {
			i := value.(*Insight)
			s.stmtCount.Add(-int64(len(i.Statements)))
			releaseInsight(i)
		},
	}

	s.mu.insights = cache.NewUnorderedCache(config)
	return s
}
