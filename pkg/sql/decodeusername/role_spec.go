// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

// Package decodeusername contains utilities for decoding usernames
// from their specs as defined in tree.
package decodeusername

import (
	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/cockroach/pkg/sql/pgwire/pgcode"
	"github.com/cockroachdb/cockroach/pkg/sql/pgwire/pgerror"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/sessiondata"
	"github.com/cockroachdb/errors"
)

// FromRoleSpec converts a tree.RoleSpec to a security.SQLUsername.
func FromRoleSpec(
	sessionData *sessiondata.SessionData, purpose username.Purpose, r tree.RoleSpec,
) (username.SQLUsername, error) {
	if r.RoleSpecType == tree.CurrentUser {
		return sessionData.User(), nil
	} else if r.RoleSpecType == tree.SessionUser {
		return sessionData.SessionUser(), nil
	}
	user, err := username.MakeSQLUsernameFromUserInput(r.Name, purpose)
	if err != nil {
		if errors.Is(err, username.ErrUsernameTooLong) {
			err = pgerror.WithCandidateCode(err, pgcode.NameTooLong)
		} else if errors.IsAny(err, username.ErrUsernameInvalid, username.ErrUsernameEmpty) {
			err = pgerror.WithCandidateCode(err, pgcode.InvalidName)
		}
		return user, errors.Wrapf(err, "%q", user)
	}
	return user, nil
}

// FromRoleSpecList converts a tree.RoleSpecList to a slice of security.SQLUsername.
func FromRoleSpecList(
	sessionData *sessiondata.SessionData, purpose username.Purpose, l tree.RoleSpecList,
) ([]username.SQLUsername, error) {
	targetRoles := make([]username.SQLUsername, len(l))
	for i, role := range l {
		user, err := FromRoleSpec(sessionData, purpose, role)
		if err != nil {
			return nil, err
		}
		targetRoles[i] = user
	}
	return targetRoles, nil
}

// FromNameList converts a tree.NameList containing SQL input of usernames,
// normalizes the names and returns them as a list of SQLUsernames.
func FromNameList(l tree.NameList) ([]username.SQLUsername, error) {
	targetRoles := make([]username.SQLUsername, len(l))
	for i, role := range l {
		user, err := username.MakeSQLUsernameFromUserInput(string(role), username.PurposeValidation)
		if err != nil {
			return nil, err
		}
		targetRoles[i] = user
	}
	return targetRoles, nil
}
