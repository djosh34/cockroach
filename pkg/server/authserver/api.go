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

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/cockroach/pkg/sql"
	"github.com/cockroachdb/cockroach/pkg/sql/isql"
)

// Server authenticates HTTP requests.
type Server interface {
	// UserFromRequest returns the user that an HTTP request authenticates
	// as, by its TLS client certificate. See (*authenticationServer).
	UserFromRequest(r *http.Request) (username.SQLUsername, error)
}

type SQLServerInterface interface {
	ExecutorConfig() *sql.ExecutorConfig
	InternalExecutor() isql.Executor
}

type AuthMux interface {
	http.Handler
}

func NewServer(cfg *base.Config, sqlServer SQLServerInterface) Server {
	return &authenticationServer{
		cfg:       cfg,
		sqlServer: sqlServer,
	}
}

// NewMux returns a handler that authenticates each request before it
// passes it to inner. If allowAnonymous is set, it passes requests without
// a user too.
func NewMux(s Server, inner http.Handler, allowAnonymous bool) AuthMux {
	return &authenticationMux{
		server:         s.(*authenticationServer),
		inner:          inner,
		allowAnonymous: allowAnonymous,
	}
}
