// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package clusterversion_test

import (
	"context"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/clusterversion"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/testutils"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/require"
)

func TestMakeMetricsAndRegisterOnVersionChangeCallback(t *testing.T) {
	defer leaktest.AfterTest(t)()
	ctx := context.Background()
	s1 := cluster.MakeTestingClusterSettingsWithVersions(clusterversion.TestingBinaryVersion, clusterversion.TestingBinaryMinSupportedVersion, true)
	s2 := cluster.MakeTestingClusterSettingsWithVersions(clusterversion.TestingBinaryVersion, clusterversion.TestingBinaryMinSupportedVersion, true)

	m1 := clusterversion.MakeMetricsAndRegisterOnVersionChangeCallback(&s1.SV)
	m2 := clusterversion.MakeMetricsAndRegisterOnVersionChangeCallback(&s2.SV)

	require.Equal(t, int64(0), m1.PreserveDowngradeLastUpdated.Value())
	require.Equal(t, int64(0), m2.PreserveDowngradeLastUpdated.Value())

	// Test that the metrics returned are independent.
	clusterversion.PreserveDowngradeVersion.Override(ctx, &s1.SV, clusterversion.TestingBinaryVersion.String())
	testutils.SucceedsSoon(t, func() error {
		if int64(0) >= m1.PreserveDowngradeLastUpdated.Value() {
			return errors.New("metric is zero. expected positive number.")
		}
		return nil
	})
	require.Equal(t, int64(0), m2.PreserveDowngradeLastUpdated.Value())

	clusterversion.PreserveDowngradeVersion.Override(ctx, &s2.SV, clusterversion.TestingBinaryVersion.String())
	testutils.SucceedsSoon(t, func() error {
		if int64(0) >= m2.PreserveDowngradeLastUpdated.Value() {
			return errors.New("metric is zero. expected positive number.")
		}
		return nil
	})

	clusterversion.PreserveDowngradeVersion.Override(ctx, &s1.SV, "")
	testutils.SucceedsSoon(t, func() error {
		if int64(0) != m1.PreserveDowngradeLastUpdated.Value() {
			return errors.New("metric is non-zero. expected zero.")
		}
		return nil
	})
}

func BenchmarkClusterVersionSettingIsActive(b *testing.B) {
	s := cluster.MakeTestingClusterSettingsWithVersions(clusterversion.TestingBinaryVersion, clusterversion.TestingBinaryMinSupportedVersion, true)
	ctx := context.Background()
	active := true
	for i := 0; i < b.N; i++ {
		active = s.Version.IsActive(ctx, clusterversion.VCurrent_Start) && active
	}
	require.True(b, active)
}
