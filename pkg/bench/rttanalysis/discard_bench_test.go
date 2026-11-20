// Copyright 2020 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package rttanalysis

import "testing"

func BenchmarkDiscard(b *testing.B) { reg.Run(b) }
func init() {
	reg.Register("Discard", []RoundTripBenchTestCase{
		{
			Name: "DISCARD ALL, no tables",
			Setup: `DROP DATABASE IF EXISTS db CASCADE; 
CREATE DATABASE db;`,
			Stmt: "DISCARD ALL",
		},
		{
			Name: "DISCARD ALL, 1 tables in 1 db",
			Setup: `
SET experimental_enable_temp_tables = true;
DROP DATABASE IF EXISTS db CASCADE; 
CREATE DATABASE db; 
USE db; 
CREATE TEMPORARY TABLE foo(i int);
`,
			Stmt: "DISCARD ALL",
		},
		{
			Name: "DISCARD ALL, 2 tables in 2 dbs",
			Setup: `
SET experimental_enable_temp_tables = true;
DROP DATABASE IF EXISTS db CASCADE; 
CREATE DATABASE db; 
USE db; 
CREATE TEMPORARY TABLE foo(i int);
CREATE TEMPORARY TABLE bar(i int);
DROP DATABASE IF EXISTS db2 CASCADE; 
CREATE DATABASE db2; 
USE db2; 
CREATE TEMPORARY TABLE foo(i int);
CREATE TEMPORARY TABLE bar(i int);
`,
			Stmt: "DISCARD ALL",
		},
	})
}
