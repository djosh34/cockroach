// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

// Package tenantcapabilities describes the privileges allotted to tenants.
package tenantcapabilities

import (
	"fmt"

	"github.com/cockroachdb/cockroach/pkg/multitenant/tenantcapabilities/tenantcapabilitiespb"
	"github.com/cockroachdb/cockroach/pkg/spanconfig/spanconfigbounds"
	"github.com/cockroachdb/redact"
	"github.com/cockroachdb/redact/interfaces"
)

// Capability is an individual capability.
type Capability interface {
	fmt.Stringer
	redact.SafeFormatter
	ID() ID
}

// TypedCapability is a convenience interface to describe the specializations
// of Capability for extracting typed values from a set of capabilities.
type TypedCapability[T any] interface {
	Capability
	Value(*tenantcapabilitiespb.TenantCapabilities) TypedValue[T]
}

type (
	BoolCapability             = TypedCapability[bool]
	SpanConfigBoundsCapability = TypedCapability[*spanconfigbounds.Bounds]
)

type boolCapability ID

func (b boolCapability) String() string                                 { return ID(b).String() }
func (b boolCapability) SafeFormat(s interfaces.SafePrinter, verb rune) { s.Print(ID(b)) }
func (b boolCapability) ID() ID                                         { return ID(b) }
func (b boolCapability) Value(t *tenantcapabilitiespb.TenantCapabilities) BoolValue {
	return MustGetValueByID(t, b.ID()).(BoolValue)
}

type spanConfigBoundsCapability ID

func (b spanConfigBoundsCapability) String() string                                 { return ID(b).String() }
func (b spanConfigBoundsCapability) SafeFormat(s interfaces.SafePrinter, verb rune) { s.Print(ID(b)) }
func (b spanConfigBoundsCapability) ID() ID                                         { return ID(b) }
func (b spanConfigBoundsCapability) Value(
	t *tenantcapabilitiespb.TenantCapabilities,
) SpanConfigBoundValue {
	return MustGetValueByID(t, b.ID()).(SpanConfigBoundValue)
}

var _ TypedCapability[bool] = boolCapability(0)
