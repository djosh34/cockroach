// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package tenantcapabilitiesauthorizer

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/kv/kvpb"
	"github.com/cockroachdb/cockroach/pkg/multitenant/tenantcapabilities"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
)

// AllowEverythingAuthorizer is a tenantcapabilities.Authorizer that
// allows all operations
type AllowEverythingAuthorizer struct{}

var _ tenantcapabilities.Authorizer = &AllowEverythingAuthorizer{}

// NewAllowEverythingAuthorizer constructs and returns a AllowEverythingAuthorizer.
func NewAllowEverythingAuthorizer() *AllowEverythingAuthorizer {
	return &AllowEverythingAuthorizer{}
}

// HasCapabilityForBatch implements the tenantcapabilities.Authorizer interface.
func (n *AllowEverythingAuthorizer) HasCapabilityForBatch(
	context.Context, roachpb.TenantID, *kvpb.BatchRequest,
) error {
	return nil
}

// BindReader implements the tenantcapabilities.Authorizer interface.
func (n *AllowEverythingAuthorizer) BindReader(tenantcapabilities.Reader) {}

// HasNodeStatusCapability implements the tenantcapabilities.Authorizer interface.
func (n *AllowEverythingAuthorizer) HasNodeStatusCapability(
	ctx context.Context, tenID roachpb.TenantID,
) error {
	return nil
}

// HasTSDBQueryCapability implements the tenantcapabilities.Authorizer interface.
func (n *AllowEverythingAuthorizer) HasTSDBQueryCapability(
	ctx context.Context, tenID roachpb.TenantID,
) error {
	return nil
}

// HasNodelocalStorageCapability implements the tenantcapabilities.Authorizer interface.
func (n *AllowEverythingAuthorizer) HasNodelocalStorageCapability(
	ctx context.Context, tenID roachpb.TenantID,
) error {
	return nil
}

// IsExemptFromRateLimiting implements the tenantcapabilities.Authorizer interface.
func (n *AllowEverythingAuthorizer) IsExemptFromRateLimiting(
	context.Context, roachpb.TenantID,
) bool {
	return true
}

// HasProcessDebugCapability implements the tenantcapabilities.Authorizer interface.
func (n *AllowEverythingAuthorizer) HasProcessDebugCapability(
	ctx context.Context, tenID roachpb.TenantID,
) error {
	return nil
}
