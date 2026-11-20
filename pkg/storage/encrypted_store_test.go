// Copyright 2019 The Cockroach Authors.
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
	"testing"

	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/pebble/vfs"
	"github.com/cockroachdb/pebble/vfs/atomicfs"
	"github.com/stretchr/testify/require"
)

// TestOpenEncryptedStore checks that a store with the registry marker that
// upstream CockroachDB writes for encryption at rest does not open.
func TestOpenEncryptedStore(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	fs := vfs.NewMem()
	require.NoError(t, fs.MkdirAll("db", 0755))
	marker, _, err := atomicfs.LocateMarker(fs, "db", registryMarkerName)
	require.NoError(t, err)
	require.NoError(t, marker.Move("COCKROACHDB_REGISTRY_000001"))
	require.NoError(t, marker.Close())

	_, err = Open(context.Background(), Location{dir: "db", fs: fs}, cluster.MakeClusterSettings())
	require.ErrorIs(t, err, errEncryptedStore)
}
