// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package descs

import (
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/lease"
	"github.com/cockroachdb/errors"
	"github.com/cockroachdb/redact"
)

// IsTwoVersionInvariantViolationError is true if the error is the
// special error returned from CheckTwoVersionInvariant.
func IsTwoVersionInvariantViolationError(err error) bool {
	return errors.HasType(err, (*twoVersionInvariantViolationError)(nil))
}

type twoVersionInvariantViolationError struct {
	ids []lease.IDVersion
}

func (t *twoVersionInvariantViolationError) Error() string {
	return redact.Sprint(t).StripMarkers()
}

func (t twoVersionInvariantViolationError) SafeFormatError(p errors.Printer) (next error) {
	p.Printf("cannot publish new versions for descriptors: "+
		"%v, old versions still in use", t.ids)
	return nil
}

var _ errors.SafeFormatter = (*twoVersionInvariantViolationError)(nil)
