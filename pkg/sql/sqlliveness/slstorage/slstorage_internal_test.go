// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package slstorage

import (
	"testing"

	"github.com/cockroachdb/cockroach/pkg/clusterversion"
	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/server/settingswatcher"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/systemschema"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/stretchr/testify/require"
)

func TestGetEncoder(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	type codecType int
	const (
		isNil codecType = iota
		isRbr
		isRbt
	)

	checkCodec := func(t *testing.T, typ codecType, codec keyCodec) {
		t.Helper()
		switch typ {
		case isNil:
			require.Nil(t, codec)
		case isRbr:
			require.NotNil(t, codec)
			_, ok := codec.(*rbrEncoder)
			require.True(t, ok, "expected %v to be an rbr encoder", codec)
		case isRbt:
			require.NotNil(t, codec)
			_, ok := codec.(*rbtEncoder)
			require.True(t, ok, "expected %v to be an rbt encoder", codec)
		}
	}

	type testCase struct {
		name      string
		version   clusterversion.Key
		readCodec codecType
		dualCodec codecType
	}
	tests := []testCase{
		{
			name:      "v23_1",
			version:   clusterversion.V23_1,
			readCodec: isRbr,
			dualCodec: isNil,
		},
		{
			name:      "v22_2",
			version:   clusterversion.V22_2,
			readCodec: isRbt,
			dualCodec: isNil,
		},
		{
			name:      "V23_1_SystemRbrDualWrite",
			version:   clusterversion.V23_1_SystemRbrDualWrite,
			readCodec: isRbt,
			dualCodec: isRbr,
		},
		{
			name:      "V23_1_SystemRbrReadNew",
			version:   clusterversion.V23_1_SystemRbrReadNew,
			readCodec: isRbr,
			dualCodec: isRbt,
		},
		{
			name:      "V23_1_SystemRbrSingleWrite",
			version:   clusterversion.V23_1_SystemRbrSingleWrite,
			readCodec: isRbr,
			dualCodec: isNil,
		},
		{
			name:      "V23_1_SystemRbrCleanup",
			version:   clusterversion.V23_1_SystemRbrCleanup,
			readCodec: isRbr,
			dualCodec: isNil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			storage := NewTestingStorage(
				log.AmbientContext{}, nil, nil, nil, keys.SystemSQLCodec, nil, nil, systemschema.SqllivenessTable(), nil)

			version := clusterversion.ClusterVersion{Version: clusterversion.ByKey(tc.version)}
			guard := settingswatcher.TestMakeVersionGuard(version)

			checkCodec(t, tc.readCodec, storage.getReadCodec(&guard))
			checkCodec(t, tc.dualCodec, storage.getDualWriteCodec(&guard))
		})
	}
}
