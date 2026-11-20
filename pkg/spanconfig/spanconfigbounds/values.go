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
	"strconv"

	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/redact"
	"github.com/cockroachdb/redact/interfaces"
)

// Value is a value from a SpanConfig that knows how to format itself.
type Value interface {
	fmt.Stringer
	redact.SafeFormatter
}

type int32Value int32

func (i int32Value) String() string {
	return fmt.Sprintf("%d", int32(i))
}
func (i int32Value) SafeFormat(s interfaces.SafePrinter, verb rune) {
	s.Printf("%d", int32(i))
}

type int64Value int64

func (i int64Value) String() string {
	return fmt.Sprintf("%d", int64(i))
}
func (i int64Value) SafeFormat(s interfaces.SafePrinter, verb rune) {
	s.Printf("%d", int64(i))
}

type constraintsConjunctionValue []roachpb.ConstraintsConjunction

func (c constraintsConjunctionValue) String() string {
	return fmt.Sprint([]roachpb.ConstraintsConjunction(c))
}
func (c constraintsConjunctionValue) SafeFormat(s interfaces.SafePrinter, verb rune) {
	s.Printf("%v", []roachpb.ConstraintsConjunction(c))
}

type leasePreferencesValue []roachpb.LeasePreference

func (l leasePreferencesValue) String() string {
	return fmt.Sprint([]roachpb.LeasePreference(l))
}
func (l leasePreferencesValue) SafeFormat(s interfaces.SafePrinter, verb rune) {
	s.Printf("%v", []roachpb.LeasePreference(l))
}

type boolValue bool

func (b boolValue) String() string {
	return strconv.FormatBool(bool(b))
}
func (b boolValue) SafeFormat(s interfaces.SafePrinter, verb rune) {
	s.Print(bool(b))
}
