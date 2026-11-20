// Copyright 2019 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package a

import "github.com/cockroachdb/errors"

type myError struct{}

func (m *myError) Error() string { return "" }

var myOtherError = errors.New("a")

func f() {
	var v error
	if v == myOtherError { // want `use errors\.Is instead of a direct comparison`
		panic("here")
	}
	// nolint:errcmp
	if v == myOtherError {
		panic("here")
	}

	switch v { // want `invalid direct comparison of error object`
	case myOtherError:
		panic("here")
	}
	// nolint:errcmp
	switch v {
	case myOtherError:
		panic("here")
	}
	switch v {
	// nolint:errcmp
	case myOtherError:
		panic("here")
	}

	switch v.(type) { // want `invalid direct cast on error object`
	case *myError:
		panic("there")
	}
	// nolint:errcmp
	switch v.(type) {
	case *myError:
		panic("there")
	}
}
