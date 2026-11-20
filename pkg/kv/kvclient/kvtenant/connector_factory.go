// Copyright 2020 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

// Package kvtenant provides utilities required by SQL-only tenant processes in
// order to interact with the key-value layer.
package kvtenant

import (
	"github.com/cockroachdb/cockroach/pkg/config/zonepb"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/rpc"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/retry"
)

// Factory creates tenant connectors. Servers accept only a loopback address;
// tests replace it with InitTestConnectorFactory.
var Factory ConnectorFactory = connectorFactory{}

// ConnectorConfig encompasses the configuration required to create a Connector.
type ConnectorConfig struct {
	TenantID          roachpb.TenantID
	AmbientCtx        log.AmbientContext
	RPCContext        *rpc.Context
	RPCRetryOptions   retry.Options
	DefaultZoneConfig *zonepb.ZoneConfig

	// ShutdownTenantConnectorEarlyIfNoRecordPresent, if set, will cause the
	// tenant connector to be shut down early if no record is present in the
	// system.tenants table. This is useful for tests that want to verify that
	// the tenant connector can't start when the record doesn't exist.
	ShutdownTenantConnectorEarlyIfNoRecordPresent bool
}

// KVAddressConfig encompasses the network addresses, pointing to KV nodes,
// required to create a Connector.
type KVAddressConfig struct {
	RemoteAddresses []string
	LoopbackAddress string
}

// ConnectorFactory constructs a new tenant Connector from the provided network
// addresses pointing to KV nodes.
type ConnectorFactory interface {
	NewConnector(cfg ConnectorConfig, addressConfig KVAddressConfig) (Connector, error)
}
