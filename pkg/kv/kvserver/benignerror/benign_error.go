// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.
package benignerror

import "github.com/cockroachdb/errors"

// BenignError may be used for declaring an error that is less serious i.e.
// benign.
type BenignError struct {
	cause error
}

// New returns a new benign error with the given error cause.
func New(cause error) *BenignError {
	return &BenignError{cause: cause}
}

func (be *BenignError) Error() string { return be.cause.Error() }
func (be *BenignError) Cause() error  { return be.cause }

func IsBenign(err error) bool {
	return errors.HasType(err, (*BenignError)(nil))
}
