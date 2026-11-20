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
	"github.com/cockroachdb/errors"
)

// AllowNothingAuthorizer is a tenantcapabilities.Authorizer that
// allows all operations
type AllowNothingAuthorizer struct{}

var _ tenantcapabilities.Authorizer = &AllowNothingAuthorizer{}

// NewAllowNothingAuthorizer constructs and returns a AllowNothingAuthorizer.
func NewAllowNothingAuthorizer() *AllowNothingAuthorizer {
	return &AllowNothingAuthorizer{}
}

// HasCapabilityForBatch implements the tenantcapabilities.Authorizer interface.
func (n *AllowNothingAuthorizer) HasCapabilityForBatch(
	context.Context, roachpb.TenantID, *kvpb.BatchRequest,
) error {
	return errors.New("operation blocked")
}

// BindReader implements the tenantcapabilities.Authorizer interface.
func (n *AllowNothingAuthorizer) BindReader(tenantcapabilities.Reader) {}

// HasNodeStatusCapability implements the tenantcapabilities.Authorizer interface.
func (n *AllowNothingAuthorizer) HasNodeStatusCapability(
	ctx context.Context, tenID roachpb.TenantID,
) error {
	return errors.New("operation blocked")
}

// HasTSDBQueryCapability implements the tenantcapabilities.Authorizer interface.
func (n *AllowNothingAuthorizer) HasTSDBQueryCapability(
	ctx context.Context, tenID roachpb.TenantID,
) error {
	return errors.New("operation blocked")
}

// HasNodelocalStorageCapability implements the tenantcapabilities.Authorizer interface.
func (n *AllowNothingAuthorizer) HasNodelocalStorageCapability(
	ctx context.Context, tenID roachpb.TenantID,
) error {
	return errors.New("operation blocked")
}

// IsExemptFromRateLimiting implements the tenantcapabilities.Authorizer interface.
func (n *AllowNothingAuthorizer) IsExemptFromRateLimiting(context.Context, roachpb.TenantID) bool {
	return false
}

// HasProcessDebugCapability implements the tenantcapabilities.Authorizer interface.
func (n *AllowNothingAuthorizer) HasProcessDebugCapability(
	ctx context.Context, tenID roachpb.TenantID,
) error {
	return errors.New("operation blocked")
}
