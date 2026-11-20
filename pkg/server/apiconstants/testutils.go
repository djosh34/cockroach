// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package apiconstants

import "github.com/cockroachdb/cockroach/pkg/security/username"

const (
	// TestingUser is a username available in test servers,
	// that has been granted the admin role.
	TestingUser = "authentic_user"

	// TestingUserNoAdmin is a username available in test servers,
	// that has not been granted the admin role.
	TestingUserNoAdmin = "authentic_user_noadmin"
)

// TestingUserName returns the username of the authenticated
// user with an admin role.
func TestingUserName() username.SQLUsername {
	return username.MakeSQLUsernameFromPreNormalizedString(TestingUser)
}

// TestingUserNameNoAdmin returns the username of the
// authenticated user without an admin role.
func TestingUserNameNoAdmin() username.SQLUsername {
	return username.MakeSQLUsernameFromPreNormalizedString(TestingUserNoAdmin)
}
