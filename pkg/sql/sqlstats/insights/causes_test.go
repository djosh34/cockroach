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
	"testing"
	"time"

	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/stretchr/testify/require"
)

func TestCauses(t *testing.T) {
	ctx := context.Background()
	st := cluster.MakeTestingClusterSettings()
	p := &causes{st: st}
	LatencyThreshold.Override(ctx, &st.SV, 100*time.Millisecond)
	HighRetryCountThreshold.Override(ctx, &st.SV, 10)

	var latencyThreshold = LatencyThreshold.Get(&st.SV)

	testCases := []struct {
		name      string
		statement *Statement
		causes    []Cause
	}{
		{
			name:      "unset",
			statement: &Statement{},
			causes:    nil,
		},
		{
			name:      "suboptimal plan",
			statement: &Statement{IndexRecommendations: []string{"THIS IS AN INDEX RECOMMENDATION"}},
			causes:    []Cause{Cause_SuboptimalPlan},
		},
		{
			name:      "high contention time",
			statement: &Statement{Contention: &latencyThreshold},
			causes:    []Cause{Cause_HighContention},
		},
		{
			name:      "high retry count",
			statement: &Statement{Retries: 10},
			causes:    []Cause{Cause_HighRetryCount},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.ElementsMatch(t, tc.causes, p.examine(nil /* buf */, tc.statement))
		})
	}
}
