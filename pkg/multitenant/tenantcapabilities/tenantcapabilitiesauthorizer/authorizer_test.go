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
	"fmt"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/kv/kvpb"
	"github.com/cockroachdb/cockroach/pkg/multitenant/tenantcapabilities"
	"github.com/cockroachdb/cockroach/pkg/multitenant/tenantcapabilities/tenantcapabilitiespb"
	"github.com/cockroachdb/cockroach/pkg/multitenant/tenantcapabilities/tenantcapabilitiestestutils"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/testutils/datapathutils"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/datadriven"
	"github.com/stretchr/testify/require"
)

// TestDataDriven runs datadriven tests against the Authorizer interface. The
// syntax is as follows:
//
// "update-state": updates the underlying global tenant capability state.
// Example:
//
// upsert ten=10 can_admin_split=true
// ----
// ok
//
// delete ten=15
// ----
// ok
//
// "has-capability-for-batch": performs a capability check, given a tenant and
// batch request declaration. Example:
//
// has-capability-for-batch ten=10 cmds=(split)
// ----
// ok
//
// "has-node-status-capability": performas a capability check to be able to
// retrieve node status metadata. Example:
//
// has-node-status-capability ten=11
// ----
// ok
//
// "has-tsdb-query-capability": performas a capability check to be able to
// make TSDB queries. Example:
//
// has-tsdb-query-capability ten=11
// ----
// ok
//
// "set-bool-cluster-setting": overrides the specified boolean cluster setting
// to the given value. Currently, only the authorizerEnabled cluster setting is
// supported.
//
// set-bool-cluster-setting name=tenant_capabilities.authorizer.enabled value=false
// ----
// ok
func TestDataDriven(t *testing.T) {
	defer leaktest.AfterTest(t)()

	datadriven.Walk(t, datapathutils.TestDataPath(t), func(t *testing.T, path string) {
		clusterSettings := cluster.MakeTestingClusterSettings()
		ctx := context.Background()
		mockReader := mockReader(make(map[roachpb.TenantID]*tenantcapabilitiespb.TenantCapabilities))
		authorizer := New(clusterSettings, nil /* TestingKnobs */)
		authorizer.BindReader(mockReader)

		datadriven.RunTest(t, path, func(t *testing.T, d *datadriven.TestData) string {
			var tenID roachpb.TenantID
			if d.HasArg("ten") {
				tenID = tenantcapabilitiestestutils.GetTenantID(t, d)
			}
			switch d.Cmd {
			case "upsert":
				_, caps, err := tenantcapabilitiestestutils.ParseTenantCapabilityUpsert(t, d)
				if err != nil {
					return err.Error()
				}
				mockReader.updateState([]*tenantcapabilities.Update{
					{
						Entry: tenantcapabilities.Entry{
							TenantID:           tenID,
							TenantCapabilities: caps,
						},
					},
				})
			case "delete":
				update := tenantcapabilitiestestutils.ParseTenantCapabilityDelete(t, d)
				mockReader.updateState([]*tenantcapabilities.Update{update})
			case "has-capability-for-batch":
				ba := tenantcapabilitiestestutils.ParseBatchRequests(t, d)
				err := authorizer.HasCapabilityForBatch(context.Background(), tenID, &ba)
				if err == nil {
					return "ok"
				}
				return err.Error()
			case "has-node-status-capability":
				err := authorizer.HasNodeStatusCapability(context.Background(), tenID)
				if err == nil {
					return "ok"
				}
				return err.Error()
			case "has-tsdb-query-capability":
				err := authorizer.HasTSDBQueryCapability(context.Background(), tenID)
				if err == nil {
					return "ok"
				}
				return err.Error()
			case "set-authorizer-mode":
				var valStr string
				d.ScanArgs(t, "value", &valStr)
				val, ok := authorizerMode.ParseEnum(valStr)
				if !ok {
					t.Fatalf("unknown authorizer mode %s", valStr)
				}
				authorizerMode.Override(ctx, &clusterSettings.SV, val)
			case "is-exempt-from-rate-limiting":
				return fmt.Sprintf("%t", authorizer.IsExemptFromRateLimiting(context.Background(), tenID))
			default:
				return fmt.Sprintf("unknown command %s", d.Cmd)
			}
			return "ok"
		})
	})
}

type mockReader map[roachpb.TenantID]*tenantcapabilitiespb.TenantCapabilities

var _ tenantcapabilities.Reader = mockReader{}

func (m mockReader) updateState(updates []*tenantcapabilities.Update) {
	for _, update := range updates {
		if update.Deleted {
			delete(m, update.TenantID)
		} else {
			m[update.TenantID] = update.TenantCapabilities
		}
	}
}

// GetCapabilities implements the tenantcapabilities.Reader interface.
func (m mockReader) GetCapabilities(
	id roachpb.TenantID,
) (*tenantcapabilitiespb.TenantCapabilities, bool) {
	cp, found := m[id]
	return cp, found
}

// GetGlobalCapabilityState implements the tenantcapabilities.Reader interface.
func (m mockReader) GetGlobalCapabilityState() map[roachpb.TenantID]*tenantcapabilitiespb.TenantCapabilities {
	return m
}

func TestAllBatchCapsAreBoolean(t *testing.T) {
	for _, capID := range reqMethodToCap {
		if capID >= tenantcapabilities.MaxCapabilityID {
			// One of the special values.
			continue
		}
		caps := &tenantcapabilitiespb.TenantCapabilities{}
		var v *tenantcapabilities.BoolValue
		require.Implements(t, v, tenantcapabilities.MustGetValueByID(caps, capID))
	}
}

func TestAllBatchRequestTypesHaveAssociatedCaps(t *testing.T) {
	for req := kvpb.Method(0); req < kvpb.NumMethods; req++ {
		_, ok := reqMethodToCap[req]
		if !ok {
			t.Errorf("no capability associated with request type %s", req)
		}
	}
}

// TestEndTxnWithCommitTrigger checks that a secondary tenant cannot commit a
// transaction with a commit trigger, which would split or merge a range
// without the capability for it.
func TestEndTxnWithCommitTrigger(t *testing.T) {
	defer leaktest.AfterTest(t)()
	ctx := context.Background()
	settings := cluster.MakeTestingClusterSettings()
	authorizer := New(settings, nil /* TestingKnobs */)
	tenID := roachpb.MustMakeTenantID(10)
	authorizer.BindReader(mockReader{tenID: &tenantcapabilitiespb.TenantCapabilities{}})

	batch := func(trigger *roachpb.InternalCommitTrigger) *kvpb.BatchRequest {
		ba := &kvpb.BatchRequest{}
		ba.Add(&kvpb.EndTxnRequest{Commit: true, InternalCommitTrigger: trigger})
		return ba
	}
	split := &roachpb.InternalCommitTrigger{SplitTrigger: &roachpb.SplitTrigger{}}
	for _, mode := range []authorizerModeType{authorizerModeOn, authorizerModeV222} {
		authorizerMode.Override(ctx, &settings.SV, int64(mode))
		require.NoError(t, authorizer.HasCapabilityForBatch(ctx, tenID, batch(nil)))
		require.ErrorContains(t, authorizer.HasCapabilityForBatch(ctx, tenID, batch(split)), "commit trigger")
		require.NoError(t, authorizer.HasCapabilityForBatch(ctx, roachpb.SystemTenantID, batch(split)))
	}
}
