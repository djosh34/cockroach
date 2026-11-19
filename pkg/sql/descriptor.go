// Copyright 2016 The Cockroach Authors.
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
	"github.com/cockroachdb/cockroach/pkg/settings"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/catpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/dbdesc"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/schemadesc"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/tabledesc"
	"github.com/cockroachdb/cockroach/pkg/sql/decodeusername"
	"github.com/cockroachdb/cockroach/pkg/sql/pgwire/pgcode"
	"github.com/cockroachdb/cockroach/pkg/sql/pgwire/pgerror"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/catconstants"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/sqlerrors"
	"github.com/cockroachdb/errors"
)

//
// This file contains routines for low-level access to stored
// descriptors.
//
// For higher levels in the SQL layer, these interface are likely not
// suitable; consider instead schema_accessors.go and resolver.go.
//

var (
	errEmptyDatabaseName = pgerror.New(pgcode.Syntax, "empty database name")
	errNoDatabase        = pgerror.New(pgcode.InvalidName, "no database specified")
	errNoSchema          = pgerror.Newf(pgcode.InvalidName, "no schema specified")
	errNoTable           = pgerror.New(pgcode.InvalidName, "no table specified")
	errNoType            = pgerror.New(pgcode.InvalidName, "no type specified")
	errNoFunction        = pgerror.New(pgcode.InvalidName, "no function specified")
	errNoMatch           = pgerror.New(pgcode.UndefinedObject, "no object matched")
)

// PublicSchemaCreatePrivilegeEnabled is the cluster setting that determines
// whether the CREATE privilege is given to the `public` role on the `public`
// schema at the time the schema is created.
var PublicSchemaCreatePrivilegeEnabled = settings.RegisterBoolSetting(
	settings.ApplicationLevel,
	"sql.auth.public_schema_create_privilege.enabled",
	"determines whether to grant all users the CREATE privileges on the public "+
		"schema when it is created",
	true,
	settings.WithPublic)

// createDatabase takes Database descriptor and creates it if needed,
// incrementing the descriptor counter. Returns true if the descriptor
// is actually created, false if it already existed, or an error if one was
// encountered. The ifNotExists flag is used to declare if the "already existed"
// state should be an error (false) or a no-op (true).
// createDatabase implements the DatabaseDescEditor interface.
func (p *planner) createDatabase(
	ctx context.Context, database *tree.CreateDatabase, jobDesc string,
) (*dbdesc.Mutable, bool, error) {

	dbName := string(database.Name)

	if dbID, err := p.Descriptors().LookupDatabaseID(ctx, p.txn, dbName); err == nil && dbID != descpb.InvalidID {
		if database.IfNotExists {
			// Check if the database is in a dropping state
			desc, err := p.Descriptors().ByID(p.txn).Get().Database(ctx, dbID)
			if err != nil {
				return nil, false, err
			}
			if desc.Dropped() {
				return nil, false, pgerror.Newf(pgcode.ObjectNotInPrerequisiteState,
					"database %q is being dropped, try again later",
					dbName)
			}
			// Noop.
			return nil, false, nil
		}
		return nil, false, sqlerrors.NewDatabaseAlreadyExistsError(dbName)
	} else if err != nil {
		return nil, false, err
	}

	id, err := p.extendedEvalCtx.DescIDGenerator.GenerateUniqueDescID(ctx)
	if err != nil {
		return nil, false, err
	}
	publicSchemaID, err := p.EvalContext().DescIDGenerator.GenerateUniqueDescID(ctx)
	if err != nil {
		return nil, false, err
	}

	owner := p.SessionData().User()
	if !database.Owner.Undefined() {
		owner, err = decodeusername.FromRoleSpec(
			p.SessionData(), username.PurposeValidation, database.Owner,
		)
		if err != nil {
			return nil, true, err
		}
	}

	db := dbdesc.NewInitial(
		id,
		string(database.Name),
		owner,
		dbdesc.WithPublicSchemaID(publicSchemaID),
	)
	includeCreatePriv := PublicSchemaCreatePrivilegeEnabled.Get(&p.execCfg.Settings.SV)
	publicSchema := schemadesc.NewBuilder(&descpb.SchemaDescriptor{
		ParentID:   id,
		Name:       catconstants.PublicSchemaName,
		ID:         publicSchemaID,
		Privileges: catpb.NewPublicSchemaPrivilegeDescriptor(owner, includeCreatePriv),
		Version:    1,
	}).BuildCreatedMutableSchema()

	if err := p.checkCanAlterToNewOwner(ctx, db, owner); err != nil {
		return nil, true, err
	}

	if err := p.createDescriptor(ctx, db, jobDesc); err != nil {
		return nil, true, err
	}
	if err := p.createDescriptor(
		ctx,
		publicSchema,
		tree.AsStringWithFQNames(database, p.Ann()),
	); err != nil {
		return nil, true, err
	}

	return db, true, nil
}

func (p *planner) createDescriptor(
	ctx context.Context, descriptor catalog.MutableDescriptor, jobDesc string,
) error {
	if err := p.shouldRestrictAccessToSystemInterface(ctx,
		"DDL execution",   /* operation */
		"running the DDL", /* alternate action */
	); err != nil {
		return err
	}

	if !descriptor.IsNew() {
		return errors.AssertionFailedf(
			"expected new descriptor, not a modification of version %d",
			descriptor.OriginalVersion())
	}
	b := p.Txn().NewBatch()
	kvTrace := p.ExtendedEvalContext().Tracing.KVTracingEnabled()
	if err := p.Descriptors().WriteDescToBatch(ctx, kvTrace, descriptor, b); err != nil {
		return err
	}
	if !descriptor.SkipNamespace() {
		if err := p.Descriptors().InsertNamespaceEntryToBatch(ctx, kvTrace, descriptor, b); err != nil {
			return err
		}
	}
	if err := p.txn.Run(ctx, b); err != nil {
		return err
	}
	if tbl, ok := descriptor.(*tabledesc.Mutable); ok && tbl.Adding() {
		// Queue a schema change job to eventually make the table public.
		if err := p.createOrUpdateSchemaChangeJob(
			ctx, tbl, jobDesc, descpb.InvalidMutationID,
		); err != nil {
			return err
		}
	}
	return nil
}

// GetImmutableTableInterfaceByID is part of the EvalPlanner interface.
func (p *planner) GetImmutableTableInterfaceByID(ctx context.Context, id int) (interface{}, error) {
	desc, err := p.Descriptors().ByIDWithLeased(p.txn).WithoutNonPublic().Get().Table(ctx, descpb.ID(id))
	if err != nil {
		return nil, err
	}
	return desc, nil
}
