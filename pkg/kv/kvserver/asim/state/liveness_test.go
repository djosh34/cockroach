// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package state

import (
	"testing"
	"time"

	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/liveness/livenesspb"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/util/hlc"
	"github.com/stretchr/testify/require"
)

func TestMockLiveness(t *testing.T) {
	clock := &ManualSimClock{nanos: time.Date(2022, 03, 21, 11, 0, 0, 0, time.UTC).UnixNano()}
	now := hlc.NewClockForTesting(clock).Now()
	testCases := []struct {
		status           livenesspb.NodeLivenessStatus
		IsAlive          bool
		MembershipStatus livenesspb.MembershipStatus
	}{
		{
			status:           livenesspb.NodeLivenessStatus_UNKNOWN,
			IsAlive:          false,
			MembershipStatus: livenesspb.MembershipStatus_ACTIVE,
		},
		{
			status:           livenesspb.NodeLivenessStatus_DEAD,
			IsAlive:          false,
			MembershipStatus: livenesspb.MembershipStatus_ACTIVE,
		},
		{
			status:           livenesspb.NodeLivenessStatus_UNAVAILABLE,
			IsAlive:          false,
			MembershipStatus: livenesspb.MembershipStatus_ACTIVE,
		},
		{
			status:           livenesspb.NodeLivenessStatus_DECOMMISSIONED,
			IsAlive:          false,
			MembershipStatus: livenesspb.MembershipStatus_DECOMMISSIONED,
		},
		{
			status:           livenesspb.NodeLivenessStatus_DECOMMISSIONING,
			IsAlive:          true,
			MembershipStatus: livenesspb.MembershipStatus_DECOMMISSIONING,
		},
		{
			status:           livenesspb.NodeLivenessStatus_LIVE,
			IsAlive:          true,
			MembershipStatus: livenesspb.MembershipStatus_ACTIVE,
		},
		{
			status:           livenesspb.NodeLivenessStatus_DRAINING,
			IsAlive:          true,
			MembershipStatus: livenesspb.MembershipStatus_ACTIVE,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.status.String(), func(t *testing.T) {
			nv := convertNodeStatusToNodeVitality(roachpb.NodeID(1), tc.status, now)
			require.Equal(t, tc.IsAlive, nv.IsLive(livenesspb.Rebalance))
			require.Equal(t, tc.MembershipStatus, nv.MembershipStatus())
			require.Equal(t, tc.status, nv.LivenessStatus())
		})
	}
}
