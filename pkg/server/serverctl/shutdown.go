// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package serverctl

import "github.com/cockroachdb/errors"

// ShutdownCause returns the reason for the shutdown request, as an error.
func (r ShutdownRequest) ShutdownCause() error {
	switch r.Reason {
	case ShutdownReasonDrainRPC:
		return errors.Newf("shutdown requested by drain RPC")
	default:
		return r.Err
	}
}

// TerminateUsingGracefulDrain determines whether the shutdown should
// be effected via a graceful drain first.
func (r ShutdownRequest) TerminateUsingGracefulDrain() bool {
	// Note: ShutdownReasonDrainRPC despite its name cannot be effected
	// via a graceful drain, because at the time it is generated
	// the RPC subsystem is shut down already.
	return r.Reason == ShutdownReasonGracefulStopRequestedByOrchestration
}

// Empty returns true if the receiver is the zero value.
func (r ShutdownRequest) Empty() bool {
	return r == (ShutdownRequest{})
}

// MakeShutdownRequest constructs a serverctl.ShutdownRequest.
func MakeShutdownRequest(reason ShutdownReason, err error) ShutdownRequest {
	if reason == ShutdownReasonDrainRPC && err != nil {
		panic(errors.NewAssertionErrorWithWrappedErrf(err, "programming error: unexpected err for ShutdownReasonDrainRPC"))
	}
	return ShutdownRequest{
		Reason: reason,
		Err:    err,
	}
}
