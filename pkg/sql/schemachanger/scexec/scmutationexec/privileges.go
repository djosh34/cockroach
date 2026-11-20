// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package scmutationexec

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/cockroach/pkg/sql/privilege"
	"github.com/cockroachdb/cockroach/pkg/sql/schemachanger/scop"
)

func (i *immediateVisitor) UpdateOwner(ctx context.Context, op scop.UpdateOwner) error {
	desc, err := i.checkOutDescriptor(ctx, op.Owner.DescriptorID)
	if err != nil {
		return err
	}
	privs := desc.GetPrivileges()
	privs.SetOwner(username.SQLUsernameProto(op.Owner.Owner).Decode())
	return nil
}

func (i *immediateVisitor) UpdateUserPrivileges(
	ctx context.Context, op scop.UpdateUserPrivileges,
) error {
	desc, err := i.checkOutDescriptor(ctx, op.Privileges.DescriptorID)
	if err != nil {
		return err
	}

	privs := desc.GetPrivileges()
	user := username.SQLUsernameProto(op.Privileges.UserName).Decode()
	privs.RemoveUser(user)
	privList, err := privilege.ListFromBitField(op.Privileges.Privileges, desc.GetObjectType())
	if err != nil {
		return err
	}
	grantList, err := privilege.ListFromBitField(op.Privileges.WithGrantOption, desc.GetObjectType())
	if err != nil {
		return err
	}
	for _, priv := range privList {
		privs.Grant(
			user,
			privilege.List{priv},
			grantList.Contains(priv),
		)
	}
	return nil
}
