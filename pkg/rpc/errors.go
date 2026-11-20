// Copyright 2023 The Cockroach Authors.
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
	"github.com/cockroachdb/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// errQuiescing is returned from client interceptors when the server's
// stopper is quiescing. The error is constructed to return true in
// `grpcutil.IsConnectionRejected` which prevents infinite retry loops during
// cluster shutdown, especially in unit testing.
var errQuiescing = status.Error(codes.PermissionDenied, "refusing to dial; node is quiescing")

// ErrNotHeartbeated is returned by ConnHealth or Connection.Health when we have
// not yet performed the first heartbeat. This error will typically only be
// observed when checking the health during the first connection attempt to a
// node, as during subsequent periods of an unhealthy connection the circuit
// breaker error will be returned instead.
var ErrNotHeartbeated = errors.New("not yet heartbeated")

type versionCompatError struct{}

func (versionCompatError) Error() string {
	return "version compatibility check failed on ping response"
}

var VersionCompatError = versionCompatError{}
