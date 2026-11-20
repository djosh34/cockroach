// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package spanconfigstore

import (
	"github.com/cockroachdb/cockroach/pkg/multitenant/tenantcapabilities"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/spanconfig/spanconfigbounds"
)

// BoundsReader maintains an in-memory view of the global SpanConfigBounds state.
//
// SpanConfigBounds are stored as tenant capabilities, the state of which is
// surfaced by the tenantcapabilities.Reader. BoundsReader serves as a narrow,
// adapter interface for the same.
type BoundsReader interface {
	// Bounds returns span config bounds set for a given tenant. If no bounds have
	// been configured for the given tenant, found returns false.
	Bounds(tenID roachpb.TenantID) (_ *spanconfigbounds.Bounds, found bool)
}

type boundsReader struct {
	capabilitiesReader tenantcapabilities.Reader
}

// NewBoundsReader constructs and returns a new BoundsReader.
func NewBoundsReader(reader tenantcapabilities.Reader) BoundsReader {
	return &boundsReader{
		capabilitiesReader: reader,
	}
}

// Bounds implements the BoundsReader interface.
func (r *boundsReader) Bounds(tenID roachpb.TenantID) (_ *spanconfigbounds.Bounds, found bool) {
	capabilities, found := r.capabilitiesReader.GetCapabilities(tenID)
	if !found {
		return nil, false
	}

	b := tenantcapabilities.MustGetValueByID(
		capabilities, tenantcapabilities.TenantSpanConfigBounds,
	).(tenantcapabilities.SpanConfigBoundValue).Get()
	return b, b != nil
}

// NewEmptyBoundsReader returns a new Reader which corresponds to an empty span config
// bounds state. It's only intended for testing.
func NewEmptyBoundsReader() BoundsReader {
	return emptyReader(true)
}

type emptyReader bool

// Bounds implements the Reader interface.
func (emptyReader) Bounds(roachpb.TenantID) (*spanconfigbounds.Bounds, bool) {
	return nil, false
}
