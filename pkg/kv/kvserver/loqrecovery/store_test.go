// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package loqrecovery

import (
	"os"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/loqrecovery/loqrecoverypb"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/uuid"
	"github.com/cockroachdb/pebble/vfs"
	"github.com/stretchr/testify/require"
)

func createSomePlan(rangeID int) loqrecoverypb.ReplicaUpdatePlan {
	planID := uuid.FastMakeV4()
	return loqrecoverypb.ReplicaUpdatePlan{
		PlanID: planID,
		Updates: []loqrecoverypb.ReplicaUpdate{
			{
				RangeID: roachpb.RangeID(rangeID),
			},
		},
	}
}

func TestWritePlan(t *testing.T) {
	defer leaktest.AfterTest(t)()

	runDiskAndInMem(t, func(t *testing.T, base string, fs vfs.FS) {
		ps := NewPlanStore(base, fs)
		plan := createSomePlan(7)
		require.NoError(t, ps.SavePlan(plan), "failed creating plan in empty store")

		readPlan, exists, err := ps.LoadPlan()
		require.NoError(t, err, "plan check failed")
		require.True(t, exists, "plan must exist")
		require.EqualValues(t, plan, readPlan, "saved are loaded plans")
	})
}

func TestWriteSamePlan(t *testing.T) {
	defer leaktest.AfterTest(t)()

	runDiskAndInMem(t, func(t *testing.T, base string, fs vfs.FS) {
		ps := NewPlanStore(base, fs)
		plan := createSomePlan(7)
		require.NoError(t, ps.SavePlan(plan), "failed creating plan in empty store")
		require.NoError(t, ps.SavePlan(plan), "writing same plan again should not generate error")
	})
}

func TestNoPlan(t *testing.T) {
	defer leaktest.AfterTest(t)()

	runDiskAndInMem(t, func(t *testing.T, base string, fs vfs.FS) {
		ps := NewPlanStore(base, fs)
		_, exists, err := ps.LoadPlan()
		require.NoError(t, err, "plan check failed")
		require.False(t, exists, "plan must be empty")
	})
}

func TestOverwritePlan(t *testing.T) {
	defer leaktest.AfterTest(t)()

	runDiskAndInMem(t, func(t *testing.T, base string, fs vfs.FS) {
		ps := NewPlanStore(base, fs)
		plan := createSomePlan(7)
		require.NoError(t, ps.SavePlan(plan), "failed creating plan in empty store")

		plan2 := createSomePlan(13)
		require.NoError(t, ps.SavePlan(plan2), "should not fail overwriting old plan")

		// Ensure that stored file was not corrupt.
		readPlan, exists, err := ps.LoadPlan()
		require.NoError(t, err, "plan check failed")
		require.True(t, exists, "plan must not be empty")
		require.EqualValues(t, plan2, readPlan, "saved are loaded plans")
	})
}

func TestRemovePlan(t *testing.T) {
	defer leaktest.AfterTest(t)()

	runDiskAndInMem(t, func(t *testing.T, base string, fs vfs.FS) {
		ps := NewPlanStore(base, fs)
		plan := createSomePlan(7)
		require.NoError(t, ps.SavePlan(plan), "failed creating plan in empty store")

		require.NoError(t, ps.RemovePlan(), "failed removing plans")

		_, exists, err := ps.LoadPlan()
		require.NoError(t, err, "plan check failed")
		require.False(t, exists, "plan must be empty after clear")
	})
}

func TestRemoveEmptyPlan(t *testing.T) {
	defer leaktest.AfterTest(t)()

	runDiskAndInMem(t, func(t *testing.T, base string, fs vfs.FS) {
		ps := NewPlanStore(base, fs)
		_, exists, err := ps.LoadPlan()
		require.NoError(t, err, "check plan existence failed")
		require.False(t, exists, "plan must be empty")

		require.NoError(t, ps.RemovePlan(), "failed removing plans")
	})
}

func runDiskAndInMem(t *testing.T, test func(*testing.T, string, vfs.FS)) {
	t.Helper()
	t.Run("disk", func(t *testing.T) {
		storeDir, err := os.MkdirTemp(".", "*")
		require.NoError(t, err, "failed to create temp dir for test")
		defer func() { _ = os.RemoveAll(storeDir) }()

		test(t, storeDir, vfs.Default)
	})
	t.Run("mem", func(t *testing.T) {
		test(t, "", vfs.NewMem())
	})
}
