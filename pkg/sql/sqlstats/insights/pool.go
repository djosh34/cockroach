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
	"sync"

	"github.com/cockroachdb/cockroach/pkg/sql/clusterunique"
)

var insightPool = sync.Pool{
	New: func() interface{} {
		return new(Insight)
	},
}

func makeInsight(sessionID clusterunique.ID, transaction *Transaction) *Insight {
	insight := insightPool.Get().(*Insight)
	*insight = Insight{
		Session:     Session{ID: sessionID},
		Transaction: transaction,
	}
	return insight
}

func releaseInsight(insight *Insight) {
	insight.Statements = insight.Statements[:0]
	*insight = Insight{Statements: insight.Statements}
	insightPool.Put(insight)
}
