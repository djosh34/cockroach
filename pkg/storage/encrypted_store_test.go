// Copyright 2019 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

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
