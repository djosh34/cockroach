// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

// Package securityassets lets tests serve certificate files from memory.
// The server reads every certificate, key and rules file through the
// active Loader.
package securityassets

import (
	"os"

	"github.com/cockroachdb/errors/oserror"
)

// Loader reads certificate and key files.
type Loader struct {
	ReadFile func(filename string) ([]byte, error)
	Stat     func(name string) (os.FileInfo, error)
	// InMemory, if set, reports whether a path is served from memory. There
	// is nothing on disk to watch for such a path.
	InMemory func(name string) bool
}

// defaultLoader uses real filesystem calls.
var defaultLoader = Loader{
	ReadFile: os.ReadFile,
	Stat:     os.Stat,
	InMemory: func(string) bool { return false },
}

// loaderImpl is used to read and stat security assets.
var loaderImpl = defaultLoader

// GetLoader returns the active asset loader.
func GetLoader() Loader {
	return loaderImpl
}

// SetLoader overrides the asset loader with the passed-in one.
func SetLoader(al Loader) {
	loaderImpl = al
}

// ResetLoader restores the asset loader to the default value.
func ResetLoader() {
	loaderImpl = defaultLoader
}

// FileExists returns true iff the target file already exists.
func (al Loader) FileExists(filename string) (bool, error) {
	_, err := al.Stat(filename)
	if err != nil {
		if oserror.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
