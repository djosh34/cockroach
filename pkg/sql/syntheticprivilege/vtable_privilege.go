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

// VirtualTablePrivilege represents privileges on virtual tables such as
// crdb_internal or pg_catalog tables.
type VirtualTablePrivilege struct {
	SchemaName string `priv:"SchemaName"`
	TableName  string `priv:"TableName"`
}

// VirtualTablePrivilegeType represents the object type for
// VirtualTablePrivilege.
const VirtualTablePrivilegeType = "VirtualTable"

var _ Object = &VirtualTablePrivilege{}

// VirtualTablePathPrefix is the prefix used for virtual table privileges in system.privileges.
const VirtualTablePathPrefix = "vtable"

// GetPath implements the Object interface.
func (p *VirtualTablePrivilege) GetPath() string {
	return fmt.Sprintf("/%s/%s/%s", VirtualTablePathPrefix, p.SchemaName, p.TableName)
}

// GetFallbackPrivileges implements the Object interface.
func (e *VirtualTablePrivilege) GetFallbackPrivileges() *catpb.PrivilegeDescriptor {
	return catpb.NewPrivilegeDescriptor(
		username.PublicRoleName(),
		privilege.List{privilege.SELECT},
		privilege.List{},
		username.NodeUserName(),
	)
}

// GetObjectType implements the Object interface.
func (p *VirtualTablePrivilege) GetObjectType() privilege.ObjectType {
	return privilege.VirtualTable
}

// GetObjectTypeString implements the Object interface.
func (p *VirtualTablePrivilege) GetObjectTypeString() string {
	return string(privilege.VirtualTable)
}

// GetName implements the Object interface.
func (p *VirtualTablePrivilege) GetName() string {
	return fmt.Sprintf("%s.%s", p.SchemaName, p.TableName)
}
