// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package testutils

import (
	"context"
	gosql "database/sql"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/sqlutils"
)

// Tenant captures per-tenant span config state and encapsulates convenient
// span config testing primitives. It's safe for concurrent use.
type Tenant struct {
	serverutils.ApplicationLayerInterface

	t          *testing.T
	userToDB   map[string]*sqlutils.SQLRunner
	curDB      *sqlutils.SQLRunner
	cleanupFns []func()
}

// Exec is a wrapper around gosql.Exec that kills the test on error. It records
// the execution timestamp for subsequent use.
func (s *Tenant) Exec(query string, args ...interface{}) {
	s.curDB.Exec(s.t, query, args...)
}

// ExecWithErr is like Exec but returns the error, if any. It records the
// execution timestamp for subsequent use.
func (s *Tenant) ExecWithErr(query string, args ...interface{}) error {
	_, err := s.curDB.DB.ExecContext(context.Background(), query, args...)
	return err
}

// Query is a wrapper around gosql.Query that kills the test on error.
func (s *Tenant) Query(query string, args ...interface{}) *gosql.Rows {
	return s.curDB.Query(s.t, query, args...)
}

// QueryWithErr is like Query but returns the error.
func (s *Tenant) QueryWithErr(query string, args ...interface{}) (*gosql.Rows, error) {
	return s.curDB.DB.QueryContext(context.Background(), query, args...)
}
