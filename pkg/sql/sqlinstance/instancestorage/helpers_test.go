// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package instancestorage

import (
	"strings"

	"github.com/cockroachdb/cockroach/pkg/sql/catalog/systemschema"
)

// GetTableSQLForDatabase is a testing helper to construct the appropriate
// table schema for a given database taking into consideration whether we're
// configured for the MR schema.
func GetTableSQLForDatabase(dbName string) string {
	return strings.Replace(systemschema.SQLInstancesTableSchema,
		`CREATE TABLE system.sql_instances`,
		`CREATE TABLE "`+dbName+`".sql_instances`, 1)
}
