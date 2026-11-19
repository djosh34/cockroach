// Copyright 2020 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package sql

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/dbdesc"
	"github.com/cockroachdb/cockroach/pkg/sql/decodeusername"
	"github.com/cockroachdb/cockroach/pkg/sql/privilege"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/util/log/eventpb"
)

type alterDatabaseOwnerNode struct {
	n    *tree.AlterDatabaseOwner
	desc *dbdesc.Mutable
}

// AlterDatabaseOwner transforms a tree.AlterDatabaseOwner into a plan node.
func (p *planner) AlterDatabaseOwner(
	ctx context.Context, n *tree.AlterDatabaseOwner,
) (planNode, error) {
	if err := checkSchemaChangeEnabled(
		ctx,
		p.ExecCfg(),
		"ALTER DATABASE",
	); err != nil {
		return nil, err
	}

	dbDesc, err := p.Descriptors().MutableByName(p.txn).Database(ctx, string(n.Name))
	if err != nil {
		return nil, err
	}

	return &alterDatabaseOwnerNode{n: n, desc: dbDesc}, nil
}

func (n *alterDatabaseOwnerNode) startExec(params runParams) error {
	newOwner, err := decodeusername.FromRoleSpec(
		params.p.SessionData(), username.PurposeValidation, n.n.Owner,
	)
	if err != nil {
		return err
	}
	oldOwner := n.desc.GetPrivileges().Owner()

	if err := params.p.checkCanAlterToNewOwner(params.ctx, n.desc, newOwner); err != nil {
		return err
	}

	// To alter the owner, the user also has to have CREATEDB privilege.
	if err := params.p.CheckGlobalPrivilegeOrRoleOption(params.ctx, privilege.CREATEDB); err != nil {
		return err
	}

	if err := params.p.setNewDatabaseOwner(params.ctx, n.desc, newOwner); err != nil {
		return err
	}

	// If the owner we want to set to is the current owner, do a no-op.
	if newOwner == oldOwner {
		return nil
	}

	if err := params.p.writeNonDropDatabaseChange(
		params.ctx,
		n.desc,
		tree.AsStringWithFQNames(n.n, params.Ann()),
	); err != nil {
		return err
	}

	return nil
}

// setNewDatabaseOwner handles setting a new database owner.
// Called in ALTER DATABASE and REASSIGN OWNED BY.
func (p *planner) setNewDatabaseOwner(
	ctx context.Context, desc catalog.MutableDescriptor, newOwner username.SQLUsername,
) error {
	privs := desc.GetPrivileges()
	privs.SetOwner(newOwner)

	// Log Alter Database Owner event. This is an auditable log event and is
	// recorded in the same transaction as the table descriptor update.
	return p.logEvent(ctx,
		desc.GetID(),
		&eventpb.AlterDatabaseOwner{
			DatabaseName: desc.GetName(),
			Owner:        newOwner.Normalized(),
		})
}

func (n *alterDatabaseOwnerNode) Next(runParams) (bool, error) { return false, nil }
func (n *alterDatabaseOwnerNode) Values() tree.Datums          { return tree.Datums{} }
func (n *alterDatabaseOwnerNode) Close(context.Context)        {}
