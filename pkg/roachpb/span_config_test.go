// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package roachpb

import (
	"testing"

	"github.com/cockroachdb/cockroach/pkg/util/hlc"
	"github.com/stretchr/testify/require"
)

func TestSpanConfigHasConfigurationChange(t *testing.T) {
	spanConfig1 := SpanConfig{
		RangeMinBytes: 1,
		RangeMaxBytes: 2,
		GCPolicy: GCPolicy{
			TTLSeconds: 10,
			ProtectionPolicies: []ProtectionPolicy{{
				ProtectedTimestamp: hlc.Timestamp{
					WallTime:  1,
					Logical:   1,
					Synthetic: false,
				},
				IgnoreIfExcludedFromBackup: false,
			}},
			IgnoreStrictEnforcement: false,
		},
		GlobalReads: false,
		NumReplicas: 3,
		NumVoters:   4,
		Constraints: []ConstraintsConjunction{{
			NumReplicas: 1,
			Constraints: []Constraint{{
				Type:  Constraint_REQUIRED,
				Key:   "a",
				Value: "b",
			}},
		}},
		VoterConstraints:      nil,
		LeasePreferences:      nil,
		RangefeedEnabled:      false,
		ExcludeDataFromBackup: false,
	}
	spanConfig1Copy := spanConfig1
	spanConfig2 := SpanConfig{
		RangeMinBytes: 1,
		RangeMaxBytes: 2,
		GCPolicy: GCPolicy{
			TTLSeconds: 10,
			ProtectionPolicies: []ProtectionPolicy{{
				ProtectedTimestamp: hlc.Timestamp{
					WallTime:  2,
					Logical:   2,
					Synthetic: false,
				},
				IgnoreIfExcludedFromBackup: false,
			}},
			IgnoreStrictEnforcement: false,
		},
		GlobalReads:           false,
		NumReplicas:           3,
		NumVoters:             4,
		Constraints:           nil,
		VoterConstraints:      nil,
		LeasePreferences:      nil,
		RangefeedEnabled:      false,
		ExcludeDataFromBackup: false,
	}
	require.NotEqual(t, spanConfig1, spanConfig2)
	require.True(t, spanConfig1.HasConfigurationChange(spanConfig2))
	// Now they are the same other than the PTS
	spanConfig2.Constraints = spanConfig1.Constraints
	require.False(t, spanConfig1.HasConfigurationChange(spanConfig2))
	// Ensure that HasConfigurationChange didn't change the spanConfig.
	require.Equal(t, spanConfig1, spanConfig1Copy)

}
