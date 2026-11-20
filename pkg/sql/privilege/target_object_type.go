// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package privilege

import "github.com/cockroachdb/errors"

// TargetObjectType represents the type of object that is
// having its default privileges altered.
type TargetObjectType uint32

var targetObjectToPrivilegeObject = map[TargetObjectType]ObjectType{
	Tables:    Table,
	Sequences: Sequence,
	Schemas:   Schema,
	Types:     Type,
	Routines:  Routine,
}

// ToObjectType returns the privilege.ObjectType corresponding to
// the TargetObjectType.
func (t TargetObjectType) ToObjectType() ObjectType {
	return targetObjectToPrivilegeObject[t]
}

// The numbers are explicitly assigned since the DefaultPrivilegesPerObject
// map defined in the DefaultPrivilegesPerRole proto requires the key value
// for the object type to remain unchanged.
const (
	Tables    TargetObjectType = 1
	Sequences TargetObjectType = 2
	Types     TargetObjectType = 3
	Schemas   TargetObjectType = 4
	Routines  TargetObjectType = 5
)

// GetTargetObjectTypes returns a slice of all the
// AlterDefaultPrivilegesTargetObjects.
func GetTargetObjectTypes() []TargetObjectType {
	return []TargetObjectType{
		Tables,
		Sequences,
		Types,
		Schemas,
		Routines,
	}
}

// String makes TargetObjectType a fmt.Stringer.
func (t TargetObjectType) String() string {
	switch t {
	case Tables:
		return "tables"
	case Sequences:
		return "sequences"
	case Types:
		return "types"
	case Schemas:
		return "schemas"
	case Routines:
		return "routines"
	default:
		panic(errors.AssertionFailedf("unknown TargetObjectType value: %d", t))
	}
}
