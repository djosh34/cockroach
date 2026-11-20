// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package vtable

// CrdbInternalBuiltinFunctionComments describes the schema of the
// crdb_internal.kv_builtin_function_comments table.
var CrdbInternalBuiltinFunctionComments = `
CREATE TABLE crdb_internal.kv_builtin_function_comments (
  oid         OID NOT NULL,
  description STRING NOT NULL
)`

// CrdbInternalCatalogComments describes the schema of the
// crdb_internal.kv_catalog_comments table.
var CrdbInternalCatalogComments = `
CREATE TABLE crdb_internal.kv_catalog_comments (
  classoid    OID NOT NULL,
  objoid      OID NOT NULL,
  objsubid    INT4 NOT NULL,
  description STRING NOT NULL
)`
