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
	"context"
	"net/http"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/sql/isql"
	"github.com/cockroachdb/cockroach/pkg/sql/roleoption"
)

type ServerV2 interface {
	http.Handler
}

type AuthV2Mux interface {
	http.Handler
}

type RoleAuthzMux interface {
	http.Handler
}

// NewV2Server creates a new ServerV2 for the given outer Server,
// and base path.
func NewV2Server(
	ctx context.Context, s SQLServerInterface, cfg *base.Config, basePath string,
) ServerV2 {
	simpleMux := http.NewServeMux()

	innerServer := NewServer(cfg, s).(*authenticationServer)
	authServer := &authenticationV2Server{
		sqlServer:  s,
		authServer: innerServer,
		mux:        simpleMux,
		ctx:        ctx,
		basePath:   basePath,
	}

	authServer.registerRoutes()
	return authServer
}

// NewV2Mux creates a new AuthV2Mux for the given ServerV2.
func NewV2Mux(s ServerV2, inner http.Handler, allowAnonymous bool) AuthV2Mux {
	as := s.(*authenticationV2Server)
	return &authenticationV2Mux{
		s:              as,
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
