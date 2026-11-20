// Copyright 2021 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package storage

import (
	"context"
	"os"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/clusterversion"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/pebble"
	"github.com/cockroachdb/pebble/vfs"
	"github.com/stretchr/testify/require"
)

func TestMinVersion(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	version1 := roachpb.Version{Major: 21, Minor: 1, Patch: 0, Internal: 122}
	version2 := roachpb.Version{Major: 21, Minor: 1, Patch: 0, Internal: 126}

	mem := vfs.NewMem()
	dir := "/foo"
	require.NoError(t, mem.MkdirAll(dir, os.ModeDir))

	// Expect !ok min version file doesn't exist.
	v, ok, err := getMinVersion(mem, dir)
	require.NoError(t, err)
	require.Equal(t, roachpb.Version{}, v)
	require.False(t, ok)

	// Expect min version to not be at least any target version.
	ok, err = MinVersionIsAtLeastTargetVersion(mem, dir, version1)
	require.NoError(t, err)
	require.False(t, ok)
	ok, err = MinVersionIsAtLeastTargetVersion(mem, dir, version2)
	require.NoError(t, err)
	require.False(t, ok)

	// Expect no error when updating min version if no file currently exists.
	require.NoError(t, writeMinVersionFile(mem, dir, version1))

	// Expect min version to be version1.
	v, ok, err = getMinVersion(mem, dir)
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, version1.Equal(v))

	// Expect min version to be at least version1 but not version2.
	ok, err = MinVersionIsAtLeastTargetVersion(mem, dir, version1)
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = MinVersionIsAtLeastTargetVersion(mem, dir, version2)
	require.NoError(t, err)
	require.False(t, ok)

	// Expect no error when updating min version to a higher version.
	require.NoError(t, writeMinVersionFile(mem, dir, version2))

	// Expect min version to be at least version1 and version2.
	ok, err = MinVersionIsAtLeastTargetVersion(mem, dir, version1)
	require.NoError(t, err)
	require.True(t, ok)
	ok, err = MinVersionIsAtLeastTargetVersion(mem, dir, version2)
	require.NoError(t, err)
	require.True(t, ok)

	// Expect min version to be version2.
	v, ok, err = getMinVersion(mem, dir)
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, version2.Equal(v))

	// Expect no-op when trying to update min version to a lower version.
	require.NoError(t, writeMinVersionFile(mem, dir, version1))
	v, ok, err = getMinVersion(mem, dir)
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, version2.Equal(v))
}

func TestSetMinVersion(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	st := cluster.MakeClusterSettings()
	p, err := Open(context.Background(), InMemory(), cluster.MakeClusterSettings(), CacheSize(0))
	require.NoError(t, err)
	defer p.Close()
	require.Equal(t, pebble.FormatFlushableIngest, p.db.FormatMajorVersion())

	ValueBlocksEnabled.Override(context.Background(), &st.SV, true)
	// Advancing the store cluster version to one that supports a new feature
	// should also advance the store's format major version.
	err = p.SetMinVersion(clusterversion.ByKey(clusterversion.V23_2_PebbleFormatDeleteSizedAndObsolete))
	require.NoError(t, err)
	require.Equal(t, pebble.FormatDeleteSizedAndObsolete, p.db.FormatMajorVersion())
}
