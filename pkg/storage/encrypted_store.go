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
