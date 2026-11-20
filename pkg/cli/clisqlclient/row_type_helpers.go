// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package clisqlclient

import (
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

// databaseTypeName returns the database type name for the given type OID.
func databaseTypeName(m *pgtype.Map, typeOID uint32) string {
	dataType, ok := m.TypeForOID(typeOID)
	if !ok {
		// TODO(rafi): remove special logic once jackc/pgtype includes these types.
		switch typeOID {
		case 1002:
			return "_CHAR"
		case 1003:
			return "_NAME"
		case 1266:
			return "TIMETZ"
		case 1270:
			return "_TIMETZ"
		default:
			return ""
		}
	}
	return strings.ToUpper(dataType.Name)
}
