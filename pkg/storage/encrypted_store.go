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
	"github.com/cockroachdb/errors"
	"github.com/cockroachdb/errors/oserror"
	"github.com/cockroachdb/pebble/vfs"
	"github.com/cockroachdb/pebble/vfs/atomicfs"
)

// registryMarkerName is the marker that upstream CockroachDB writes in the
// store directory when the store uses encryption at rest.
const registryMarkerName = "registry"

// errEncryptedStore is returned when opening a store that upstream
// CockroachDB encrypted at rest.
var errEncryptedStore = errors.New("encrypted stores are unsupported")

// checkNotEncrypted returns errEncryptedStore if the store in dbDir was
// encrypted at rest by upstream CockroachDB.
func checkNotEncrypted(fs vfs.FS, dbDir string) error {
	filename, err := atomicfs.ReadMarker(fs, dbDir, registryMarkerName)
	if oserror.IsNotExist(err) {
		return nil
	} else if err != nil {
		return err
	}
	if filename != "" {
		return errEncryptedStore
	}
	return nil
}
