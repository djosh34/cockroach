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

import "github.com/cockroachdb/cockroach/pkg/settings/cluster"

type causes struct {
	st *cluster.Settings
}

// examine will append all causes of the statement's problems to buf and
// return the result. Buf allows the slice to be pooled.
func (c *causes) examine(buf []Cause, stmt *Statement) (result []Cause) {
	result = buf
	if len(stmt.IndexRecommendations) > 0 {
		result = append(result, Cause_SuboptimalPlan)
	}

	if stmt.Contention != nil && *stmt.Contention >= LatencyThreshold.Get(&c.st.SV) {
		result = append(result, Cause_HighContention)
	}

	if stmt.Retries >= HighRetryCountThreshold.Get(&c.st.SV) {
		result = append(result, Cause_HighRetryCount)
	}

	return result
}
