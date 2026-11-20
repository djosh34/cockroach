// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package rpc

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/multitenant/tenantcapabilities"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/settings"
	"google.golang.org/grpc"
)

// WrappedServerStream is exported for testing.
type WrappedServerStream = wrappedServerStream

// TestingNewWrappedServerStream constructs a WrappedServerStream for testing.
// This function returns the more generic interface to defeat the linter
// which complains about returning an unexported type. Callers can type
// assert the return value into the above-exported *WrappedServerStream.
func TestingNewWrappedServerStream(
	ctx context.Context, ss grpc.ServerStream, recvFunc func(interface{}) error,
) grpc.ServerStream {
	return &WrappedServerStream{
		ServerStream: ss,
		ctx:          ctx,
		recv:         recvFunc,
	}
}

// TestingAuthenticateTenant performs authentication of a tenant from a context
// for testing.
func TestingAuthenticateTenant(
	ctx context.Context, serverTenantID roachpb.TenantID,
) (roachpb.TenantID, error) {
	_, authz, err := kvAuth{tenant: tenantAuthorizer{tenantID: serverTenantID}}.authenticateAndSelectAuthzRule(ctx)
	if err != nil {
		return roachpb.TenantID{}, err
	}
	switch z := authz.(type) {
	case authzTenantServerToKVServer:
		return roachpb.TenantID(z), nil
	default:
		return roachpb.TenantID{}, nil
	}
}

// TestingAuthorizeTenantRequest performs authorization of a tenant request
// for testing.
func TestingAuthorizeTenantRequest(
	ctx context.Context,
	sv *settings.Values,
	tenID roachpb.TenantID,
	method string,
	request interface{},
	authorizer tenantcapabilities.Authorizer,
) error {
	return tenantAuthorizer{
		tenantID:               tenID,
		capabilitiesAuthorizer: authorizer,
	}.authorize(ctx, sv, tenID, method, request)
}
