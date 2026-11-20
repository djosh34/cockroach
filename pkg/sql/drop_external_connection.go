// Copyright 2022 The Cockroach Authors.
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

	"github.com/cockroachdb/cockroach/pkg/sql/privilege"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/sessiondata"
	"github.com/cockroachdb/cockroach/pkg/sql/syntheticprivilege"
	"github.com/cockroachdb/errors"
)

const dropExternalConnectionOp = "DROP EXTERNAL CONNECTION"

type dropExternalConnectionNode struct {
	n *tree.DropExternalConnection
}

// DropExternalConnection represents a DROP EXTERNAL CONNECTION statement.
func (p *planner) DropExternalConnection(
	_ context.Context, n *tree.DropExternalConnection,
) (planNode, error) {
	return &dropExternalConnectionNode{n: n}, nil
}

func (c *dropExternalConnectionNode) startExec(params runParams) error {
	return params.p.dropExternalConnection(params, c.n)
}

func (p *planner) dropExternalConnection(params runParams, n *tree.DropExternalConnection) error {
	// TODO(adityamaru): Add some metrics to track DROP EXTERNAL CONNECTION
	// usage.

	name, err := p.ExprEvaluator(externalConnectionOp).String(
		params.ctx, n.ConnectionLabel,
	)
	if err != nil {
		return errors.Wrap(err, "failed to resolve External Connection name")
	}

	// Check that the user has DROP privileges on the External Connection object.
	ecPrivilege := &syntheticprivilege.ExternalConnectionPrivilege{
		ConnectionName: name,
	}
	if err := p.CheckPrivilege(params.ctx, ecPrivilege, privilege.DROP); err != nil {
		return err
	}

	// DROP EXTERNAL CONNECTION is only allowed for users with the `DROP`
	// privilege on this object. We run the query as `node` since the user might
	// not have `SELECT` on the system table.
	if _ /* rows */, err = params.p.InternalSQLTxn().ExecEx(
		params.ctx,
		dropExternalConnectionOp,
		params.p.Txn(),
		sessiondata.NodeUserSessionDataOverride,
		`DELETE FROM system.external_connections WHERE connection_name = $1`, name,
	); err != nil {
		return errors.Wrapf(err, "failed to delete external connection")
	}

	// We must also DELETE all rows from system.privileges that refer to
	// external connection.
	if _, err = params.p.InternalSQLTxn().ExecEx(
		params.ctx,
		dropExternalConnectionOp,
		params.p.Txn(),
		sessiondata.NodeUserSessionDataOverride,
		`DELETE FROM system.privileges WHERE path = $1`, ecPrivilege.GetPath(),
	); err != nil {
		return errors.Wrapf(err, "failed to delete external connection")
	}

	return nil
}

func (c *dropExternalConnectionNode) Next(_ runParams) (bool, error) { return false, nil }
func (c *dropExternalConnectionNode) Values() tree.Datums            { return nil }
func (c *dropExternalConnectionNode) Close(_ context.Context)        {}
