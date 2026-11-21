// Copyright 2021 The Cockroach Authors.
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

	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/cockroach/pkg/server/srverrors"
	"github.com/cockroachdb/cockroach/pkg/sql/isql"
	"github.com/cockroachdb/cockroach/pkg/sql/roleoption"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/sessiondata"
	"github.com/cockroachdb/errors"
)

// authenticationV2Mux provides authentication checks for an arbitrary inner
// http.Handler. If the request has no valid client certificate, an HTTP
// 401 error is returned and the request isn't routed through to the inner
// handler. On success, the username is set on the request context for use
// in the inner handler.
type authenticationV2Mux struct {
	server         *authenticationServer
	inner          http.Handler
	allowAnonymous bool
}

func (a *authenticationV2Mux) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var u username.SQLUsername
	if a.allowAnonymous {
		u = username.RootUserName()
	} else {
		var err error
		if u, err = a.server.UserFromRequest(req); err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
	}
	req = req.WithContext(ContextWithHTTPAuthInfo(req.Context(), u.Normalized(), 0))
	a.inner.ServeHTTP(w, req)
}

// APIRole is an enum representing the authorization level
// needed for an APIv2 endpoint.
type APIRole int

const (
	// RegularRole is the default role for an APIv2 endpoint.
	RegularRole APIRole = iota
	// AdminRole is the role for an APIv2 endpoint that requires
	// admin privileges.
	AdminRole
	// SuperUserRole is the role for an APIv2 endpoint that requires
	// superuser privileges.
	SuperUserRole
)

// roleAuthorizationMux enforces a role (eg. type of user, role option)
// for an arbitrary inner mux. Meant to be used under authenticationV2Mux. If
// the logged-in user is not at least of `role` type, and doesn't have
// the `option` roleoption, an HTTP 403 forbidden error is returned. Otherwise,
// the request is passed onto the inner http.Handler.
type roleAuthorizationMux struct {
	ie     isql.Executor
	role   APIRole
	option roleoption.Option
	inner  http.Handler
}

func (r *roleAuthorizationMux) getRoleForUser(
	ctx context.Context, user username.SQLUsername,
) (APIRole, error) {
	if user.IsRootUser() {
		// Shortcut.
		return SuperUserRole, nil
	}
	row, err := r.ie.QueryRowEx(
		ctx, "check-is-admin", nil, /* txn */
		sessiondata.InternalExecutorOverride{User: user},
		"SELECT crdb_internal.is_admin()")
	if err != nil {
		return RegularRole, err
	}
	if row == nil {
		return RegularRole, errors.AssertionFailedf("hasAdminRole: expected 1 row, got 0")
	}
	if len(row) != 1 {
		return RegularRole, errors.AssertionFailedf("hasAdminRole: expected 1 column, got %d", len(row))
	}
	dbDatum, ok := tree.AsDBool(row[0])
	if !ok {
		return RegularRole, errors.AssertionFailedf("hasAdminRole: expected bool, got %T", row[0])
	}
	if dbDatum {
		return AdminRole, nil
	}
	return RegularRole, nil
}

func (r *roleAuthorizationMux) hasRoleOption(
	ctx context.Context, user username.SQLUsername, roleOption roleoption.Option,
) (bool, error) {
	if user.IsRootUser() {
		// Shortcut.
		return true, nil
	}
	row, err := r.ie.QueryRowEx(
		ctx, "check-role-option", nil, /* txn */
		sessiondata.InternalExecutorOverride{User: user},
		"SELECT crdb_internal.has_role_option($1)", roleOption.String())
	if err != nil {
		return false, err
	}
	if row == nil {
		return false, errors.AssertionFailedf("hasRoleOption: expected 1 row, got 0")
	}
	if len(row) != 1 {
		return false, errors.AssertionFailedf("hasRoleOption: expected 1 column, got %d", len(row))
	}
	dbDatum, ok := tree.AsDBool(row[0])
	if !ok {
		return false, errors.AssertionFailedf("hasRoleOption: expected bool, got %T", row[0])
	}
	return bool(dbDatum), nil
}

func (r *roleAuthorizationMux) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// The username is set in authenticationV2Mux, and must correspond with a
	// logged-in user.
	username := UserFromHTTPAuthInfoContext(req.Context())
	if role, err := r.getRoleForUser(req.Context(), username); err != nil || role < r.role {
		if err != nil {
			srverrors.APIV2InternalError(req.Context(), err, w)
		} else {
			http.Error(w, "user not allowed to access this endpoint", http.StatusForbidden)
		}
		return
	}
	if r.option > 0 {
		ok, err := r.hasRoleOption(req.Context(), username, r.option)
		if err != nil {
			srverrors.APIV2InternalError(req.Context(), err, w)
			return
		} else if !ok {
			http.Error(w, "user not allowed to access this endpoint", http.StatusForbidden)
			return
		}
	}
	r.inner.ServeHTTP(w, req)
}
