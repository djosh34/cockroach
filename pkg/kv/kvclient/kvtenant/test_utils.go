// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package kvtenant

// InitTestConnectorFactory replaces the kv connector factory with a lenient factory that does no filtering
// on KV Addresses. This is useful for tests that do not want to be restricted to a loopback only address.
func InitTestConnectorFactory() {
	Factory = testConnectorFactory{}
}

// testConnectorFactory is a connector factory that does no filtering on KV Addresses.
type testConnectorFactory struct{}

// NewConnector creates a new connector with the given configuration.
func (testConnectorFactory) NewConnector(
	cfg ConnectorConfig, addressConfig KVAddressConfig,
) (Connector, error) {
	return NewConnector(cfg, CombineKVAddresses(addressConfig)), nil
}
