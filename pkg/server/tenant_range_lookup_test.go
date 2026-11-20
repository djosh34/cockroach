// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package server

import (
	"context"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/util/encoding"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/stretchr/testify/require"
)

// TestFilterRangeLookupResponseForTenant unit tests the logic to filter
// RangeLookup connector requests.
func TestFilterRangeLookupResponseForTenant(t *testing.T) {
	defer leaktest.AfterTest(t)()

	ctx := context.Background()
	mkKey := func(tenant uint64, str string) roachpb.RKey {
		codec := keys.MakeSQLCodec(roachpb.MustMakeTenantID(tenant))
		return encoding.EncodeStringAscending(codec.TenantPrefix(), str)
	}
	mkRangeDescriptor := func(start, end roachpb.RKey) roachpb.RangeDescriptor {
		return *roachpb.NewRangeDescriptor(1, start, end, roachpb.MakeReplicaSet(nil))
	}
	for _, tc := range []struct {
		name              string
		id                roachpb.TenantID
		descs             []roachpb.RangeDescriptor
		exp               int
		skipTenantContext bool
	}{
		// tenant 1, the "system tenant" can see everything.
		{
			name: "tenant 1 is special",
			id:   roachpb.MustMakeTenantID(1),
			descs: []roachpb.RangeDescriptor{
				mkRangeDescriptor(mkKey(1, "a"), mkKey(1, "b")),
				mkRangeDescriptor(mkKey(1, "b"), mkKey(1, "z")),
				mkRangeDescriptor(mkKey(2, "z"), mkKey(2, "")),
				mkRangeDescriptor(mkKey(2, ""), mkKey(2, "a")),
				mkRangeDescriptor(mkKey(2, "a"), mkKey(3, "")),
				mkRangeDescriptor(mkKey(3, ""), mkKey(4, "")),
			},
			exp: 6,
		},
		// tenant 2 is a normal secondary tenant and can only see its own data.
		{
			name: "filter to tenant data",
			id:   roachpb.MustMakeTenantID(2),
			descs: []roachpb.RangeDescriptor{
				mkRangeDescriptor(mkKey(2, "a"), mkKey(2, "b")),
				mkRangeDescriptor(mkKey(2, "b"), mkKey(2, "z")),
				mkRangeDescriptor(mkKey(2, "z"), mkKey(3, "")),
				mkRangeDescriptor(mkKey(3, ""), mkKey(3, "a")),
			},
			exp: 3,
		},
		// tenant 2 is a normal secondary tenant and can only see its own data,
		// but this includes the case where the range overlaps with multiple
		// tenants.
		{
			name: "filter to tenant data even though range crosses tenants",
			id:   roachpb.MustMakeTenantID(2),
			descs: []roachpb.RangeDescriptor{
				mkRangeDescriptor(mkKey(2, "a"), mkKey(2, "b")),
				mkRangeDescriptor(mkKey(2, "b"), mkKey(2, "z")),
				mkRangeDescriptor(mkKey(2, "z"), mkKey(4, "")),
				mkRangeDescriptor(mkKey(4, ""), mkKey(4, "a")),
			},
			exp: 3,
		},
		// If there is no tenant ID in the context, only one result should be
		// returned.
		{
			id: roachpb.MustMakeTenantID(2),
			descs: []roachpb.RangeDescriptor{
				mkRangeDescriptor(mkKey(2, "a"), mkKey(2, "b")),
				mkRangeDescriptor(mkKey(2, "b"), mkKey(2, "z")),
				mkRangeDescriptor(mkKey(2, "z"), mkKey(3, "")),
				mkRangeDescriptor(mkKey(3, ""), mkKey(3, "a")),
			},
			skipTenantContext: true,
			exp:               0,
		},
		// Other code should prevent a request that might return descriptors from
		// another tenant, however, defensively this code should also filter them.
		{
			id: roachpb.MustMakeTenantID(3),
			descs: []roachpb.RangeDescriptor{
				mkRangeDescriptor(mkKey(2, "a"), mkKey(2, "b")),
				mkRangeDescriptor(mkKey(2, "b"), mkKey(2, "z")),
				mkRangeDescriptor(mkKey(2, "z"), mkKey(3, "")),
				mkRangeDescriptor(mkKey(3, ""), mkKey(3, "a")),
			},
			exp: 0,
		},
	} {
		tenantCtx := ctx
		if !tc.skipTenantContext {
			tenantCtx = roachpb.ContextWithClientTenant(ctx, tc.id)
		}
		got := filterRangeLookupResponseForTenant(tenantCtx, tc.descs)
		require.Len(t, got, tc.exp)
		require.Equal(t, tc.descs[:tc.exp], got)
	}
}
