// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package spanconfigbounds

import (
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/cockroachdb/redact"
	"github.com/cockroachdb/redact/interfaces"
)

// Violation represents a violation of the SpanConfigBound
type Violation struct {
	Field            Field
	Bounds           ValueBounds
	Value, ClampedTo Value
}

func (v Violation) SafeFormat(s interfaces.SafePrinter, verb rune) {
	s.Printf("%v: %v does not conform to %v, will be clamped to %v",
		v.Field, v.Value, v.Bounds, v.ClampedTo)
}

type Violations []Violation

func (v Violations) SafeFormat(s interfaces.SafePrinter, verb rune) {
	s.SafeRune('[')
	for i, violation := range v {
		if i > 0 {
			s.SafeString("; ")
		}
		s.Printf("%v", violation)
	}
	s.SafeRune(']')
}

func (v Violations) String() string {
	return redact.Sprint(v).StripMarkers()
}

var _ redact.SafeFormatter = Violations(nil)
var _ redact.SafeFormatter = (*Violation)(nil)

type ViolationError struct {
	Violations Violations
}

func (b *ViolationError) Format(f fmt.State, verb rune) {
	errors.FormatError(b, f, verb)
}

func (b *ViolationError) SafeFormatError(p errors.Printer) (next error) {
	p.Printf("span config bounds violated for fields: ")
	for i, v := range b.Violations {
		if i > 0 {
			p.Printf(", ")
		}
		p.Print(v.Field)
	}
	if !p.Detail() {
		return nil
	}
	for i, v := range b.Violations {
		if i > 0 {
			p.Printf("\n")
		}
		p.Print(v)
	}
	return nil
}

func (b *ViolationError) Error() string {
	return fmt.Sprintf("%v", b)
}

var _ errors.SafeFormatter = (*ViolationError)(nil)
var _ fmt.Formatter = (*ViolationError)(nil)

func (v Violations) AsError() error {
	return &ViolationError{
		Violations: v,
	}
}
