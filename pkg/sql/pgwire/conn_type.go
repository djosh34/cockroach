// Copyright 2026 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package pgwire

// ConnType is the type of a client connection.
type ConnType int

const (
	// ConnLocal is a unix socket connection.
	ConnLocal ConnType = iota + 1
	// ConnHostNoSSL is a TCP connection without TLS.
	ConnHostNoSSL
	// ConnHostSSL is a TCP connection with TLS.
	ConnHostSSL
	// ConnInternalLoopback is an internal connection over the in-process
	// loopback listener.
	ConnInternalLoopback
)

// String implements the fmt.Stringer interface.
func (t ConnType) String() string {
	switch t {
	case ConnLocal:
		return "local"
	case ConnHostNoSSL:
		return "hostnossl"
	case ConnHostSSL:
		return "hostssl"
	case ConnInternalLoopback:
		return "loopback"
	default:
		return "unknown"
	}
}
