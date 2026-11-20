// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package syntheticprivilege

import (
	"fmt"

	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/catpb"
	"github.com/cockroachdb/cockroach/pkg/sql/privilege"
)

// ExternalConnectionPrivilege represents privileges on external connection
// objects stored in `system.external_connections`.
type ExternalConnectionPrivilege struct {
	ConnectionName string `priv:"ConnectionName"`
}

var _ Object = &ExternalConnectionPrivilege{}

// GetPath implements the Object interface.
func (e *ExternalConnectionPrivilege) GetPath() string {
	return fmt.Sprintf("/externalconn/%s", e.ConnectionName)
}

// GetFallbackPrivileges implements the Object interface.
func (e *ExternalConnectionPrivilege) GetFallbackPrivileges() *catpb.PrivilegeDescriptor {
	return catpb.NewPrivilegeDescriptor(
		username.PublicRoleName(),
		privilege.List{privilege.USAGE},
		privilege.List{},
		username.NodeUserName(),
	)
}

// GetObjectType implements the Object interface.
func (e *ExternalConnectionPrivilege) GetObjectType() privilege.ObjectType {
	return privilege.ExternalConnection
}

// GetObjectTypeString implements the Object interface.
func (e *ExternalConnectionPrivilege) GetObjectTypeString() string {
	return string(privilege.ExternalConnection)
}

// GetName implements the Object interface.
func (e *ExternalConnectionPrivilege) GetName() string {
	return e.ConnectionName
}
