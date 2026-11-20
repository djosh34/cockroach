// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package op

import (
	"time"

	"github.com/cockroachdb/errors"
)

// ControlledOperation is an operation that is being managed by the Controller.
// It maintains a set of methods to inspect the lifecycle of an operation.
type ControlledOperation interface {
	// Next returns the next time that this operation should be checked.
	Next() time.Time
	// Done returns whether this controlled operation is done, if so, when the
	// operation finished.
	Done() (bool, time.Time)
	// Errors returns any errors that were recorded for this operation, if any
	// exist, otherwise nil.
	Errors() error
}

type baseOp struct {
	start, next, complete time.Time
	done                  bool
	errs                  []error
}

func newBaseOp(tick time.Time) baseOp {
	return baseOp{
		start:    tick,
		next:     tick,
		complete: time.Time{},
		errs:     []error{},
	}
}

// Next returns the next time that this operation should be checked.
func (bo baseOp) Next() time.Time {
	return bo.next
}

// Done returns whether this controlled operation is done, if so, when the
// operation finished.
func (bo baseOp) Done() (bool, time.Time) {
	return bo.done, bo.complete
}

// Errors returns any errors that were recorded for this operation, if any
// exist, otherwise nil.
func (bo baseOp) Errors() error {
	var err error
	for i := range bo.errs {
		if i == 0 {
			err = bo.errs[i]
		} else {
			err = errors.CombineErrors(err, bo.errs[i])
		}
	}
	return err
}
