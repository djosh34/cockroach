// Copyright 2020 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package fs

import (
	"io"

	"github.com/cockroachdb/pebble/vfs"
)

// CreateWithSync creates a file wrapped with logic to periodically sync
// whenever more than bytesPerSync bytes accumulate. This syncing does not
// provide any persistency guarantees, but can prevent latency spikes.
func CreateWithSync(fs vfs.FS, name string, bytesPerSync int) (vfs.File, error) {
	f, err := fs.Create(name)
	if err != nil {
		return nil, err
	}
	return vfs.NewSyncingFile(f, vfs.SyncingFileOptions{BytesPerSync: bytesPerSync}), nil
}

// WriteFile writes data to a file named by filename.
func WriteFile(fs vfs.FS, filename string, data []byte) error {
	f, err := fs.Create(filename)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err1 := f.Close(); err == nil {
		err = err1
	}
	return err
}

// ReadFile reads data from a file named by filename.
func ReadFile(fs vfs.FS, filename string) ([]byte, error) {
	file, err := fs.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(file)
}
