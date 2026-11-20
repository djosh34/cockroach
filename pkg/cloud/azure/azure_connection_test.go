// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package azure

import (
	"testing"

	"github.com/cockroachdb/cockroach/pkg/cloud/externalconn"
	"github.com/cockroachdb/cockroach/pkg/cloud/externalconn/connectionpb"
	"github.com/stretchr/testify/require"
)

func TestAzureStorageConnection(t *testing.T) {
	require.Equal(t, connectionpb.ConnectionProvider_azure_storage, externalconn.ProviderForURI("azure://test"))
	require.Equal(t, connectionpb.ConnectionProvider_azure_storage, externalconn.ProviderForURI("azure-storage://test"))
	require.Equal(t, connectionpb.ConnectionProvider_azure_storage, externalconn.ProviderForURI("azure-blob://test"))
}
