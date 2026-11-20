// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package externalconn

import (
	"context"
	"net/url"

	"github.com/cockroachdb/cockroach/pkg/cloud/externalconn/connectionpb"
)

// ExternalConnection is the interface to the external resource represented by
// an External Connection object. This interface should expose read-only
// methods that are required to interact with the External Connection object.
type ExternalConnection interface {
	// UnredactedConnectionStatement returns a `CREATE EXTERNAL CONNECTION`
	// statement that is functionally equivalent to the statement that created the
	// external connection in the first place.
	//
	// NB: The returned string will contain unredacted secrets and should not be
	// persisted.
	UnredactedConnectionStatement() string
	// ConnectionName returns the label of the connection.
	ConnectionName() string
	// ConnectionType returns the type of the connection.
	ConnectionType() connectionpb.ConnectionType
	// ConnectionProto returns an in-memory representation of the
	// ConnectionDetails that describe the underlying resource. These details
	// should not be persisted directly, refer to
	// `externalconn.NewMutableExternalConnection` for mutating an External
	// Connection object.
	ConnectionProto() *connectionpb.ConnectionDetails
}

// connectionParserFactory is the factory method that takes in an endpoint URI
// for an external resource, and returns the ExternalConnection representation
// of that URI.
type connectionParserFactory func(ctx context.Context, env ExternalConnEnv, url *url.URL) (ExternalConnection, error)
