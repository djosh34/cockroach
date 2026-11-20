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
	"testing"

	"github.com/cockroachdb/cockroach/pkg/clusterversion"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/stretchr/testify/require"
)

func TestVersionCompatibility(t *testing.T) {
	for _, d := range []struct {
		name         string
		version      roachpb.Version
		minSupported roachpb.Version
		current      roachpb.Version
		err          bool
		isV1         bool
	}{
		{
			name: "current version",
			version: roachpb.Version{
				Major: 23,
				Minor: 1,
			},
			minSupported: roachpb.Version{
				Major: 22,
				Minor: 2,
			},
			current: roachpb.Version{
				Major: 23,
				Minor: 1,
			},
		},
		{
			name: "v1",
			version: roachpb.Version{
				Major: 22,
				Minor: 2,
			},
			minSupported: roachpb.Version{
				Major: 22,
				Minor: 2,
			},
			current: roachpb.Version{
				Major: 23,
				Minor: 2,
			},
			isV1: true,
		},
		{
			name: "in between",
			version: roachpb.Version{
				Major: 23,
				Minor: 1,
			},
			minSupported: roachpb.Version{
				Major: 22,
				Minor: 2,
			},
			current: roachpb.Version{
				Major: 23,
				Minor: 2,
			},
		},
		{
			name: "too old",
			version: roachpb.Version{
				Major: 22,
				Minor: 1,
			},
			minSupported: roachpb.Version{
				Major: 22,
				Minor: 2,
			},
			current: roachpb.Version{
				Major: 23,
				Minor: 1,
			},
			err: true,
		},
		{
			name: "dev build with release data file",
			version: roachpb.Version{
				Major:    22,
				Minor:    2,
				Internal: 50,
			},
			minSupported: roachpb.Version{
				Major: 22 + clusterversion.DevOffset,
				Minor: 2,
			},
			current: roachpb.Version{
				Major:    22 + clusterversion.DevOffset,
				Minor:    2,
				Internal: 50,
			},
			err: true,
		},
		{
			name: "dev build with dev data file",
			version: roachpb.Version{
				Major:    22 + clusterversion.DevOffset,
				Minor:    2,
				Internal: 50,
			},
			minSupported: roachpb.Version{
				Major: 22 + clusterversion.DevOffset,
				Minor: 2,
			},
			current: roachpb.Version{
				Major:    22 + clusterversion.DevOffset,
				Minor:    2,
				Internal: 50,
			},
		},
		{
			name: "upgrade from v1",
			version: roachpb.Version{
				Major:    22,
				Minor:    2,
				Internal: 1,
			},
			minSupported: roachpb.Version{
				Major: 22,
				Minor: 2,
			},
			current: roachpb.Version{
				Major:    22,
				Minor:    2,
				Internal: 50,
			},
		},
	} {
		t.Run(d.name, func(t *testing.T) {
			err := checkVersionAllowedImpl(d.version, d.minSupported, d.current)
			if d.err {
				require.Error(t, err, "version check must fail")
			} else {
				require.NoError(t, err, "version check must pass")
			}
		})
	}
}

func TestPlanVersionMatching(t *testing.T) {
	for _, d := range []struct {
		name           string
		plan, cluster  roachpb.Version
		ignoreInternal bool
		err            string
	}{
		{
			name: "matching",
			plan: roachpb.Version{
				Major:    23,
				Minor:    1,
				Internal: 2,
			},
			cluster: roachpb.Version{
				Major:    23,
				Minor:    1,
				Internal: 2,
			},
		},
		{
			name: "non-matching",
			plan: roachpb.Version{
				Major: 23,
				Minor: 1,
			},
			cluster: roachpb.Version{
				Major: 23,
				Minor: 2,
			},
			err: "doesn't match cluster active version",
		},
		{
			name: "non-matching patch",
			plan: roachpb.Version{
				Major: 23,
				Minor: 1,
				Patch: 4,
			},
			cluster: roachpb.Version{
				Major: 23,
				Minor: 1,
			},
			err: "recovery plan must not use patch versions",
		},
		{
			name: "non-matching internal",
			plan: roachpb.Version{
				Major:    23,
				Minor:    1,
				Internal: 2,
			},
			cluster: roachpb.Version{
				Major:    23,
				Minor:    1,
				Internal: 4,
			},
			err: "doesn't match cluster active version",
		},
		{
			name: "ignore internal",
			plan: roachpb.Version{
				Major:    23,
				Minor:    1,
				Internal: 2,
			},
			cluster: roachpb.Version{
				Major:    23,
				Minor:    1,
				Internal: 4,
			},
			ignoreInternal: true,
		},
		{
			name: "ignore with wrong version",
			plan: roachpb.Version{
				Major:    23,
				Minor:    1,
				Internal: 2,
			},
			cluster: roachpb.Version{
				Major:    23,
				Minor:    2,
				Internal: 4,
			},
			ignoreInternal: true,
			err:            "doesn't match cluster active version",
		},
	} {
		t.Run(d.name, func(t *testing.T) {
			if d.err != "" {
				require.ErrorContains(t, checkPlanVersionMatches(d.plan, d.cluster, d.ignoreInternal),
					d.err)
			} else {
				require.NoError(t, checkPlanVersionMatches(d.plan, d.cluster, d.ignoreInternal))
			}
		})
	}
}
