// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package securityassets

import (
	"io/ioutil"
	"os"

	"github.com/cockroachdb/errors/oserror"
)

// Loader describes the functions necessary to read certificate and key files.
type Loader struct {
	ReadDir  func(dirname string) ([]os.FileInfo, error)
	ReadFile func(filename string) ([]byte, error)
	Stat     func(name string) (os.FileInfo, error)
}

// defaultLoader uses real filesystem calls.
var defaultLoader = Loader{
	ReadDir:  ioutil.ReadDir,
	ReadFile: os.ReadFile,
	Stat:     os.Stat,
}

// loaderImpl is used to list/read/stat security assets.
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
