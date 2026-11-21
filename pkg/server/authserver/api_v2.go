// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package authserver

import (
	"net/http"

	"github.com/cockroachdb/cockroach/pkg/sql/isql"
	"github.com/cockroachdb/cockroach/pkg/sql/roleoption"
)

type RoleAuthzMux interface {
	http.Handler
}

// NewV2Mux returns a handler for the v2 API that authenticates each
// request by its client certificate, like NewMux. If allowAnonymous is set,
// as in insecure mode, every request runs as root without a check.
func NewV2Mux(s Server, inner http.Handler, allowAnonymous bool) http.Handler {
	return &authenticationV2Mux{
		server:         s.(*authenticationServer),
		inner:          inner,
		allowAnonymous: allowAnonymous,
	}
}

// NewRoleAuthzMux creates a new RoleAuthzMux.
func NewRoleAuthzMux(
	ie isql.Executor, role APIRole, option roleoption.Option, inner http.Handler,
) RoleAuthzMux {
	return &roleAuthorizationMux{
		ie:     ie,
		role:   role,
		option: option,
		inner:  inner,
	}
}
