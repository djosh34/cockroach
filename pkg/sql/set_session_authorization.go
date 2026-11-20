// Copyright 2019 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package sql

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
)

func (p *planner) SetSessionAuthorizationDefault() (planNode, error) {
	return &setSessionAuthorizationDefaultNode{}, nil
}

type setSessionAuthorizationDefaultNode struct{}

func (n *setSessionAuthorizationDefaultNode) Next(_ runParams) (bool, error) { return false, nil }
func (n *setSessionAuthorizationDefaultNode) Values() tree.Datums            { return nil }
func (n *setSessionAuthorizationDefaultNode) Close(_ context.Context)        {}
func (n *setSessionAuthorizationDefaultNode) startExec(params runParams) error {
	// This is currently the same as `SET ROLE = DEFAULT`, which means that it
	// only changes the "current user." In Postgres, `SET SESSION AUTHORIZATION`
	// also changes the "session user," but since the session user cannot be
	// modified in CockroachDB (at the time of writing), we just need to change
	// the current user here.
	// NOTE: If in the future we do allow the session user to be modified, we
	// should still track the original logged-user, and use that for the audit
	// logs written by event_log.
	return params.p.setRole(params.ctx, false /* local */, params.p.SessionData().SessionUser())
}
