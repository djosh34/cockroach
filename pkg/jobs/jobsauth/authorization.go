// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package jobsauth

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/jobs/jobspb"
	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/cockroach/pkg/sql/pgwire/pgcode"
	"github.com/cockroachdb/cockroach/pkg/sql/pgwire/pgerror"
	"github.com/cockroachdb/cockroach/pkg/sql/privilege"
	"github.com/cockroachdb/cockroach/pkg/sql/roleoption"
	"github.com/cockroachdb/cockroach/pkg/sql/syntheticprivilege"
)

// An AccessLevel is used to indicate how strict an authorization check should
// be.
type AccessLevel int

const (
	// ViewAccess is used to perform authorization for viewing jobs (ex. SHOW JOBS).
	ViewAccess AccessLevel = iota

	// ControlAccess is used to perform authorization for modifying jobs (ex. PAUSE|CANCEL|RESUME JOB).
	ControlAccess
)

// AuthorizationAccessor is an interface for checking authorization on jobs.
type AuthorizationAccessor interface {
	// HasPrivilege mirrors sql.AuthorizationAccessor.
	HasPrivilege(ctx context.Context, privilegeObject privilege.Object, privilege privilege.Kind, user username.SQLUsername) (bool, error)

	// HasRoleOption mirrors sql.AuthorizationAccessor.
	HasRoleOption(ctx context.Context, roleOption roleoption.Option) (bool, error)

	// UserHasAdminRole mirrors sql.AuthorizationAccessor.
	UserHasAdminRole(ctx context.Context, user username.SQLUsername) (bool, error)

	// HasAdminRole mirrors sql.AuthorizationAccessor.
	HasAdminRole(ctx context.Context) (bool, error)

	// HasGlobalPrivilegeOrRoleOption mirrors sql.AuthorizationAccessor.
	HasGlobalPrivilegeOrRoleOption(ctx context.Context, privilege privilege.Kind) (bool, error)

	// User mirrors sql.PlanHookState.
	User() username.SQLUsername
}

// Authorize returns nil if the user is authorized to access the job.
// If the user is not authorized, then a pgcode.InsufficientPrivilege
// error will be returned.
//
// TODO(#96432): sort out internal job owners and rules for accessing them
// Authorize checks these rules in order:
//  1. If the user is an admin, grant access.
//  2. If the AccessLevel is ViewAccess, grant access if the user has CONTROLJOB, VIEWJOB,
//     or if the user owns the job.
//  3. If the AccessLevel is ControlAccess, grant access if the user has CONTROLJOB
//     and the job owner is not an admin.
//  4. If there is an authorization check for this job type that passes, grant the user access.
func Authorize(
	ctx context.Context,
	a AuthorizationAccessor,
	jobID jobspb.JobID,
	payload *jobspb.Payload,
	accessLevel AccessLevel,
) error {
	userIsAdmin, err := a.HasAdminRole(ctx)
	if err != nil {
		return err
	}
	if userIsAdmin {
		return nil
	}

	hasControlJob, err := a.HasGlobalPrivilegeOrRoleOption(ctx, privilege.CONTROLJOB)
	if err != nil {
		return err
	}

	jobOwnerUser := payload.UsernameProto.Decode()

	if accessLevel == ViewAccess {
		if a.User() == jobOwnerUser || hasControlJob {
			return nil
		}
		hasViewJob, err := a.HasPrivilege(ctx, &syntheticprivilege.GlobalPrivilege{}, privilege.VIEWJOB, a.User())
		if err != nil {
			return err
		}
		if hasViewJob {
			return nil
		}
	}

	jobOwnerIsAdmin, err := a.UserHasAdminRole(ctx, jobOwnerUser)
	if err != nil {
		return err
	}
	if jobOwnerIsAdmin {
		return pgerror.Newf(pgcode.InsufficientPrivilege,
			"only admins can control jobs owned by other admins")
	}
	if hasControlJob {
		return nil
	}

	return pgerror.Newf(pgcode.InsufficientPrivilege,
		"user %s does not have privileges for job %d",
		a.User(), jobID)
}
