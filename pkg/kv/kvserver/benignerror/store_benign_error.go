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

// StoreBenignError may be used for declaring an error that is less serious i.e.
// benign, originated from a store, and requires a retry. These errors are
// tracked in a metric away from other benign errors.
type StoreBenignError struct {
	cause error
}

// NewStoreBenign returns a new store benign error with the given error cause.
func NewStoreBenign(cause error) *StoreBenignError {
	return &StoreBenignError{cause: cause}
}

func (be *StoreBenignError) Error() string { return be.cause.Error() }
func (be *StoreBenignError) Cause() error  { return be.cause }

func IsStoreBenign(err error) bool {
	return errors.HasType(err, (*StoreBenignError)(nil))
}
