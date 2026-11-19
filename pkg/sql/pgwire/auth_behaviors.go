// Copyright 2021 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package pgwire

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/errors"
)

// AuthBehaviors encapsulates the per-connection behaviors that may be
// configured. This type is returned by AuthMethod implementations.
type AuthBehaviors struct {
	authenticator       Authenticator
	replacementIdentity username.SQLUsername
	replacedIdentity    bool
	roleMapper          RoleMapper
}

// Ensure that an AuthBehaviors is easily composable with itself.
var _ Authenticator = (*AuthBehaviors)(nil).Authenticate
var _ RoleMapper = (*AuthBehaviors)(nil).MapRole

// Authenticate delegates to the Authenticator passed to
// SetAuthenticator or returns an error if SetAuthenticator has not been
// called.
func (b *AuthBehaviors) Authenticate(
	ctx context.Context,
	systemIdentity username.SQLUsername,
	clientConnection bool,
	pwRetrieveFn PasswordRetrievalFn,
) error {
	if found := b.authenticator; found != nil {
		return found(ctx, systemIdentity, clientConnection, pwRetrieveFn)
	}
	return errors.New("no Authenticator provided to AuthBehaviors")
}

// SetAuthenticator updates the Authenticator to be used.
func (b *AuthBehaviors) SetAuthenticator(a Authenticator) {
	b.authenticator = a
}

// ReplacementIdentity returns an optional replacement for the
// client-provided identity when validating the incoming connection.
// The cert method uses this to replace it with the certificate's common
// name. This method will return ok==false if
// SetReplacementIdentity has not been called.
func (b *AuthBehaviors) ReplacementIdentity() (_ username.SQLUsername, ok bool) {
	return b.replacementIdentity, b.replacedIdentity
}

// SetReplacementIdentity allows the AuthMethod to override the
// client-reported system identity.
func (b *AuthBehaviors) SetReplacementIdentity(id username.SQLUsername) {
	b.replacementIdentity = id
	b.replacedIdentity = true
}

// MapRole delegates to the RoleMapper passed to SetRoleMapper or
// returns an error if SetRoleMapper has not been called.
func (b *AuthBehaviors) MapRole(
	ctx context.Context, systemIdentity username.SQLUsername,
) ([]username.SQLUsername, error) {
	if found := b.roleMapper; found != nil {
		return found(ctx, systemIdentity)
	}
	return nil, errors.New("no RoleMapper provided to AuthBehaviors")
}

// SetRoleMapper updates the RoleMapper to be used.
func (b *AuthBehaviors) SetRoleMapper(m RoleMapper) {
	b.roleMapper = m
}
