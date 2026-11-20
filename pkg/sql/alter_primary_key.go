// Copyright 2020 The Cockroach Authors.
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
	"time"

	"github.com/cockroachdb/cockroach/pkg/server/telemetry"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/catenumpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/catsessiondata"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descs"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/tabledesc"
	"github.com/cockroachdb/cockroach/pkg/sql/paramparse"
	"github.com/cockroachdb/cockroach/pkg/sql/pgwire/pgcode"
	"github.com/cockroachdb/cockroach/pkg/sql/pgwire/pgerror"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/sqlerrors"
	"github.com/cockroachdb/cockroach/pkg/sql/sqltelemetry"
	"github.com/cockroachdb/cockroach/pkg/util/errorutil/unimplemented"
	"github.com/cockroachdb/errors"
)

func (p *planner) AlterPrimaryKey(
	ctx context.Context, tableDesc *tabledesc.Mutable, alterPKNode tree.AlterTableAlterPrimaryKey,
) error {
	if err := paramparse.ValidateUniqueConstraintParams(
		alterPKNode.StorageParams,
		paramparse.UniqueConstraintParamContext{
			IsPrimaryKey: true,
			IsSharded:    alterPKNode.Sharded != nil,
		},
	); err != nil {
		return err
	}

	// Ensure that other schema changes on this table are not currently
	// executing, and that other schema changes have not been performed
	// in the current transaction.
	currentMutationID := tableDesc.ClusterVersion().NextMutationID
	for i := range tableDesc.Mutations {
		mut := &tableDesc.Mutations[i]
		if mut.MutationID == currentMutationID {
			return unimplemented.NewWithIssuef(
				45510,
				"cannot perform a primary key change on %[1]s with other schema changes on %[1]s in the same transaction",
				tableDesc.Name,
			)
		}
		if mut.MutationID < currentMutationID {
			// We can handle indexes being deleted concurrently. We do this
			// in order to not be blocked on index drops created by a previous
			// primary key change. If we errored out when seeing a previous
			// index drop, then users would see a confusing message that a
			// schema change is in progress when it doesn't seem like one is.
			// TODO (rohany): This feels like such a hack until (#45510) is fixed.
			if mut.GetIndex() != nil && mut.Direction == descpb.DescriptorMutation_DROP {
				continue
			}
			return unimplemented.NewWithIssuef(
				45510, "table %s is currently undergoing a schema change", tableDesc.Name)
		}
	}

	for _, elem := range alterPKNode.Columns {
		if elem.Column == "" && elem.Expr != nil {
			return errors.WithHint(
				pgerror.Newf(
					pgcode.InvalidColumnDefinition,
					"expressions such as %q are not allowed in primary index definition",
					elem.Expr.String(),
				),
				"use columns instead",
			)
		}
		col, err := catalog.MustFindColumnByTreeName(tableDesc, elem.Column)
		if err != nil {
			return err
		}
		if col.Dropped() {
			return pgerror.Newf(pgcode.ObjectNotInPrerequisiteState,
				"column %q is being dropped", col.GetName())
		}
		if col.IsInaccessible() {
			return pgerror.Newf(pgcode.InvalidSchemaDefinition, "cannot use inaccessible column %q in primary key", col.GetName())
		}
		if col.IsNullable() {
			return pgerror.Newf(pgcode.InvalidSchemaDefinition, "cannot use nullable column %q in primary key", col.GetName())
		}
	}

	// Validate if the end result is the same as the current
	// primary index, which would mean nothing needs to be modified
	// here.
	{
		requiresIndexChange, err := p.shouldCreateIndexes(ctx, tableDesc, &alterPKNode)
		if err != nil {
			return err
		}
		if !requiresIndexChange {
			return nil
		}
	}

	nameExists := func(name string) bool {
		return catalog.FindIndexByName(tableDesc, name) != nil
	}

	// Make a new index that is suitable to be a primary index.
	name := tabledesc.GenerateUniqueName(
		"new_primary_key",
		nameExists,
	)
	if alterPKNode.Name != "" &&
		// Allow reuse of existing primary key's name.
		tableDesc.PrimaryIndex.Name != string(alterPKNode.Name) &&
		nameExists(string(alterPKNode.Name)) {
		return pgerror.Newf(pgcode.DuplicateRelation, "index with name %s already exists", alterPKNode.Name)
	}
	newPrimaryIndexDesc := &descpb.IndexDescriptor{
		Name:              name,
		Unique:            true,
		CreatedExplicitly: true,
		EncodingType:      catenumpb.PrimaryIndexEncoding,
		Type:              descpb.IndexDescriptor_FORWARD,
		// TODO(postamar): bump version to LatestIndexDescriptorVersion in 22.2
		// This is not possible until then because of a limitation in 21.2 which
		// affects mixed-21.2-22.1-version clusters (issue #78426).
		Version:        descpb.StrictIndexColumnIDGuaranteesVersion,
		ConstraintID:   tableDesc.GetNextConstraintID(),
		CreatedAtNanos: p.EvalContext().GetTxnTimestamp(time.Microsecond).UnixNano(),
	}
	tableDesc.NextConstraintID++

	// If the new index is requested to be sharded, set up the index descriptor
	// to be sharded, and add the new shard column if it is missing.
	if alterPKNode.Sharded != nil {
		shardCol, newColumns, err := setupShardedIndex(
			ctx,
			p.EvalContext(),
			&p.semaCtx,
			alterPKNode.Columns,
			alterPKNode.Sharded.ShardBuckets,
			tableDesc,
			newPrimaryIndexDesc,
			alterPKNode.StorageParams,
			false, /* isNewTable */
		)
		if err != nil {
			return err
		}
		alterPKNode.Columns = newColumns
		if err := p.maybeSetupConstraintForShard(
			ctx,
			tableDesc,
			shardCol,
			newPrimaryIndexDesc.Sharded.ShardBuckets,
		); err != nil {
			return err
		}
		telemetry.Inc(sqltelemetry.HashShardedIndexCounter)
	}

	if err := newPrimaryIndexDesc.FillColumns(alterPKNode.Columns); err != nil {
		return err
	}

	{
		// Add all deletable non-virtual non-pk columns to new primary index.
		names := make(map[string]struct{}, len(newPrimaryIndexDesc.KeyColumnNames))
		for _, name := range newPrimaryIndexDesc.KeyColumnNames {
			names[name] = struct{}{}
		}
		deletable := tableDesc.DeletableColumns()
		newPrimaryIndexDesc.StoreColumnNames = make([]string, 0, len(deletable))
		for _, col := range deletable {
			if _, found := names[col.GetName()]; found || col.IsVirtual() {
				continue
			}
			newPrimaryIndexDesc.StoreColumnNames = append(newPrimaryIndexDesc.StoreColumnNames, col.GetName())
		}
		if len(newPrimaryIndexDesc.StoreColumnNames) == 0 {
			newPrimaryIndexDesc.StoreColumnNames = nil
		}
	}

	if err := tableDesc.AddIndexMutationMaybeWithTempIndex(
		newPrimaryIndexDesc, descpb.DescriptorMutation_ADD,
	); err != nil {
		return err
	}
	version := p.ExecCfg().Settings.Version.ActiveVersion(ctx)
	if err := tableDesc.AllocateIDs(ctx, version); err != nil {
		return err
	}

	var err error
	// We have to rewrite all indexes that either:
	// * depend on uniqueness from the old primary key (inverted, non-unique, or unique with nulls).
	// * don't store or index all columns in the new primary key.
	shouldRewriteIndex := func(idx catalog.Index) (bool, error) {
		colIDs := idx.CollectKeyColumnIDs()
		if !idx.Primary() {
			colIDs.UnionWith(idx.CollectSecondaryStoredColumnIDs())
			colIDs.UnionWith(idx.CollectKeySuffixColumnIDs())
		}
		for _, colID := range newPrimaryIndexDesc.KeyColumnIDs {
			if !colIDs.Contains(colID) {
				return true, nil
			}
		}
		if !idx.Primary() && catalog.MakeTableColSet(newPrimaryIndexDesc.KeyColumnIDs...).SubsetOf(
			catalog.MakeTableColSet(tableDesc.PrimaryIndex.KeyColumnIDs...)) {
			// Always rewrite a secondary index if the new PK columns is a (strict) subset of the old PK columns.
			return true, nil
		}
		if idx.IsUnique() {
			for i := 0; i < idx.NumKeyColumns(); i++ {
				colID := idx.GetKeyColumnID(i)
				col, err := catalog.MustFindColumnByID(tableDesc, colID)
				if err != nil {
					return false, err
				}
				if col.IsNullable() {
					return true, nil
				}
			}
		}

		// If there is any virtual column of the secondary index not a primary key
		// column, we should rewrite the index as well because we don't allow
		// non-primary key virtual column in secondary index's suffix columns, and
		// this would be violated if we don't rewrite the index.
		if !idx.Primary() {
			newPrimaryKeyColIDs := catalog.MakeTableColSet(newPrimaryIndexDesc.KeyColumnIDs...)
			for i := 0; i < idx.NumKeySuffixColumns(); i++ {
				colID := idx.GetKeySuffixColumnID(i)
				col, err := catalog.MustFindColumnByID(tableDesc, colID)
				if err != nil {
					return false, err
				}
				if col.IsVirtual() && !newPrimaryKeyColIDs.Contains(colID) {
					return true, err
				}
			}
		}

		return !idx.IsUnique() || idx.GetType() == descpb.IndexDescriptor_INVERTED, nil
	}
	var indexesToRewrite []catalog.Index
	for _, idx := range tableDesc.PublicNonPrimaryIndexes() {
		shouldRewrite, err := shouldRewriteIndex(idx)
		if err != nil {
			return err
		}
		if idx.GetID() != newPrimaryIndexDesc.ID && shouldRewrite {
			indexesToRewrite = append(indexesToRewrite, idx)
		}
		// If this index is referenced by any other objects, then we wil
		// block the primary key swap, since we don't have a mechanism to
		// fix these references yet.
		for _, tableRef := range tableDesc.GetDependedOnBy() {
			if tableRef.IndexID == idx.GetID() {
				refDesc, err := p.Descriptors().ByIDWithLeased(p.txn).Get().Desc(ctx, tableRef.ID)
				if err != nil {
					return err
				}
				return unimplemented.NewWithIssuef(124131,
					"table %q has an index (%s) that is still referenced by %q",
					tableDesc.GetName(),
					idx.GetName(),
					refDesc.GetName())
			}
		}
	}

	if err := p.disallowDroppingPrimaryIndexReferencedInUDFOrView(ctx, tableDesc); err != nil {
		return err
	}

	// TODO (rohany): this loop will be unused until #45510 is resolved.
	for _, mut := range tableDesc.AllMutations() {
		// If there is an index that is getting built right now that started in a previous txn, we
		// need to potentially rebuild that index as well.
		if idx := mut.AsIndex(); mut.MutationID() < currentMutationID && idx != nil && mut.Adding() {
			shouldRewrite, err := shouldRewriteIndex(idx)
			if err != nil {
				return err
			}
			if shouldRewrite {
				indexesToRewrite = append(indexesToRewrite, idx)
			}
		}
	}

	// Queue up a mutation for each index that needs to be rewritten.
	// This new index will have an altered KeySuffixColumnIDs to allow it to be
	// rewritten using the unique-ifying columns from the new table.
	var oldIndexIDs, newIndexIDs []descpb.IndexID
	for _, idx := range indexesToRewrite {
		// Clone the index that we want to rewrite.
		newIndex := idx.IndexDescDeepCopy()
		basename := newIndex.Name + "_rewrite_for_primary_key_change"

		newIndex.Name = tabledesc.GenerateUniqueName(basename, nameExists)
		// TODO(postamar): bump version to LatestIndexDescriptorVersion in 22.2
		// This is not possible until then because of a limitation in 21.2 which
		// affects mixed-21.2-22.1-version clusters (issue #78426).
		newIndex.Version = descpb.StrictIndexColumnIDGuaranteesVersion
		newIndex.EncodingType = catenumpb.SecondaryIndexEncoding
		if err := addIndexMutationWithSpecificPrimaryKey(ctx, tableDesc, &newIndex, newPrimaryIndexDesc, p.ExecCfg().Settings); err != nil {
			return err
		}

		oldIndexIDs = append(oldIndexIDs, idx.GetID())
		newIndexIDs = append(newIndexIDs, newIndex.ID)
	}

	// Create a new index that indexes everything the old primary index
	// does, but doesn't store anything.
	if shouldCopyPrimaryKey(tableDesc, newPrimaryIndexDesc) {
		newUniqueIdx := tableDesc.GetPrimaryIndex().IndexDescDeepCopy()
		// Clear the following fields so that they get generated by AllocateIDs.
		newUniqueIdx.ID = 0
		newUniqueIdx.Name = ""
		newUniqueIdx.StoreColumnIDs = nil
		newUniqueIdx.StoreColumnNames = nil
		newUniqueIdx.KeySuffixColumnIDs = nil
		newUniqueIdx.CompositeColumnIDs = nil
		newUniqueIdx.KeyColumnIDs = nil
		// Set correct version and encoding type.
		//
		// TODO(postamar): bump version to LatestIndexDescriptorVersion in 22.2
		// This is not possible until then because of a limitation in 21.2 which
		// affects mixed-21.2-22.1-version clusters (issue #78426).
		newUniqueIdx.Version = descpb.StrictIndexColumnIDGuaranteesVersion
		newUniqueIdx.EncodingType = catenumpb.SecondaryIndexEncoding
		if err := addIndexMutationWithSpecificPrimaryKey(ctx, tableDesc, &newUniqueIdx, newPrimaryIndexDesc, p.ExecCfg().Settings); err != nil {
			return err
		}
	}

	// Determine if removing this index would lead to the uniqueness for a foreign
	// key back reference, which will cause this swap operation to be blocked.
	const withSearchForReplacement = true
	err = p.tryRemoveFKBackReferences(
		ctx,
		tableDesc,
		tableDesc.GetPrimaryIndex(),
		tree.DropRestrict,
		withSearchForReplacement,
	)
	if err != nil {
		return err
	}

	swapArgs := &descpb.PrimaryKeySwap{
		OldPrimaryIndexId:   tableDesc.GetPrimaryIndexID(),
		NewPrimaryIndexId:   newPrimaryIndexDesc.ID,
		NewIndexes:          newIndexIDs,
		OldIndexes:          oldIndexIDs,
		NewPrimaryIndexName: string(alterPKNode.Name),
	}
	tableDesc.AddPrimaryKeySwapMutation(swapArgs)

	dvmp := catsessiondata.NewDescriptorSessionDataProvider(p.SessionData())
	if err := descs.ValidateSelf(tableDesc, version, dvmp); err != nil {
		return err
	}

	// Mark the primary key of the table as valid.
	{
		primaryIndex := *tableDesc.GetPrimaryIndex().IndexDesc()
		primaryIndex.Disabled = false
		tableDesc.SetPrimaryIndex(primaryIndex)
	}

	// N.B. We don't schedule index deletions here because the existing
	// indexes need to be visible to the user until the primary key swap
	// actually occurs. Deletions will get enqueued in the phase when
	// the swap happens.

	return nil
}

// Given the current table descriptor and the new primary keys
// index descriptor  this function determines if the two are
// equivalent and if any index creation operations are needed
// by comparing properties.
func (p *planner) shouldCreateIndexes(
	ctx context.Context, desc *tabledesc.Mutable, alterPKNode *tree.AlterTableAlterPrimaryKey,
) (requiresIndexChange bool, err error) {
	oldPK := desc.GetPrimaryIndex()

	// Validate if basic properties between the two match.
	if oldPK.NumKeyColumns() != len(alterPKNode.Columns) ||
		oldPK.IsSharded() != (alterPKNode.Sharded != nil) {
		return true, nil
	}

	// Validate if sharding properties are the same.
	if alterPKNode.Sharded != nil {
		shardBuckets, err := tabledesc.EvalShardBucketCount(ctx, &p.semaCtx, p.EvalContext(), alterPKNode.Sharded.ShardBuckets, alterPKNode.StorageParams)
		if err != nil {
			return true, err
		}
		if oldPK.GetSharded().ShardBuckets != shardBuckets {
			return true, nil
		}
	}

	// If the old primary key is dropped, then recreation
	// is required.
	if oldPK.IsDisabled() {
		return true, nil
	}

	// Validate the columns on the indexes
	for idx, elem := range alterPKNode.Columns {
		col, err := catalog.MustFindColumnByTreeName(desc, elem.Column)
		if err != nil {
			return true, err
		}

		if col.GetID() != oldPK.GetKeyColumnID(idx) {
			return true, nil
		}
		if (elem.Direction == tree.Ascending &&
			oldPK.GetKeyColumnDirection(idx) != catenumpb.IndexColumn_ASC) ||
			(elem.Direction == tree.Descending &&
				oldPK.GetKeyColumnDirection(idx) != catenumpb.IndexColumn_DESC) {
			return true, nil
		}
	}
	return false, nil
}

// We only recreate the old primary key of the table as a unique secondary
// index if:
//   - The table has a primary key (no DROP PRIMARY KEY statements have
//     been executed).
//   - The primary key is not the default rowid primary key.
//   - The new primary key isn't the same set of columns and directions
//     other than hash sharding.
//   - There is no existing secondary index on the old primary key columns.
func shouldCopyPrimaryKey(desc *tabledesc.Mutable, newPK *descpb.IndexDescriptor) bool {

	columnIDsAndDirsWithoutSharded := func(idx *descpb.IndexDescriptor) (
		columnIDs descpb.ColumnIDs,
		columnDirs []catenumpb.IndexColumn_Direction,
	) {
		for i, colName := range idx.KeyColumnNames {
			if colName != idx.Sharded.Name {
				columnIDs = append(columnIDs, idx.KeyColumnIDs[i])
				columnDirs = append(columnDirs, idx.KeyColumnDirections[i])
			}
		}
		return columnIDs, columnDirs
	}
	idsAndDirsMatch := func(old, new *descpb.IndexDescriptor) bool {
		oldIDs, oldDirs := columnIDsAndDirsWithoutSharded(old)
		newIDs, newDirs := columnIDsAndDirsWithoutSharded(new)
		if !oldIDs.Equals(newIDs) {
			return false
		}
		for i := range oldDirs {
			if oldDirs[i] != newDirs[i] {
				return false
			}
		}
		return true
	}
	alreadyHasSecondaryIndexOnPKColumns := func(desc catalog.TableDescriptor) bool {
		// Return true if there exists a secondary index that is "identical" to desc's PK index. This function
		// is used to avoid creating duplicate secondary index during `ALTER PRIMARY KEY`.

		// Two indexes are considered "identical" if they have the same
		// uniqueness AND partiallness AND same key columns in same order with same direction
		isIdentical := func(idx1, idx2 catalog.Index) bool {
			if idx1.IsUnique() != idx2.IsUnique() ||
				idx1.IsPartial() != idx2.IsPartial() ||
				idx1.NumKeyColumns() != idx2.NumKeyColumns() ||
				(idx1.IsPartial() && idx1.GetPredicate() != idx2.GetPredicate()) {
				return false
			}

			for i := 0; i < idx1.NumKeyColumns(); i++ {
				if idx1.GetKeyColumnID(i) != idx2.GetKeyColumnID(i) ||
					idx1.GetKeyColumnDirection(i) != idx2.GetKeyColumnDirection(i) {
					return false
				}
			}

			return true
		}

		pk := desc.GetPrimaryIndex()
		for _, idx := range desc.PublicNonPrimaryIndexes() {
			if isIdentical(pk, idx) {
				return true
			}
		}
		return false
	}
	oldPK := desc.GetPrimaryIndex().IndexDesc()
	return desc.HasPrimaryKey() &&
		!desc.IsPrimaryIndexDefaultRowID() &&
		!idsAndDirsMatch(oldPK, newPK) &&
		!alreadyHasSecondaryIndexOnPKColumns(desc)
}

// addIndexMutationWithSpecificPrimaryKey adds an index mutation into the given
// table descriptor, but sets up the index with KeySuffixColumnIDs from the
// given index, rather than the table's primary key.
func addIndexMutationWithSpecificPrimaryKey(
	ctx context.Context,
	table *tabledesc.Mutable,
	toAdd *descpb.IndexDescriptor,
	primary *descpb.IndexDescriptor,
	settings *cluster.Settings,
) error {
	// Reset the ID so that a call to AllocateIDs will set up the index.
	toAdd.ID = 0
	toAdd.ConstraintID = 0
	if err := table.AddIndexMutationMaybeWithTempIndex(toAdd, descpb.DescriptorMutation_ADD); err != nil {
		return err
	}
	if err := table.AllocateIDsWithoutValidation(ctx); err != nil {
		return err
	}

	if err := setKeySuffixColumnIDsFromPrimary(table, toAdd, primary); err != nil {
		return err
	}
	if tempIdx := catalog.FindCorrespondingTemporaryIndexByID(table, toAdd.ID); tempIdx != nil {
		if err := setKeySuffixColumnIDsFromPrimary(table, tempIdx.IndexDesc(), primary); err != nil {
			return err
		}
	}
	return nil
}

// setKeySuffixColumnIDsFromPrimary uses the columns in the given
// primary index to construct this toAdd's KeySuffixColumnIDs list.
func setKeySuffixColumnIDsFromPrimary(
	table *tabledesc.Mutable, toAdd *descpb.IndexDescriptor, primary *descpb.IndexDescriptor,
) error {
	presentColIDs := catalog.MakeTableColSet(toAdd.KeyColumnIDs...)
	presentColIDs.UnionWith(catalog.MakeTableColSet(toAdd.StoreColumnIDs...))
	toAdd.KeySuffixColumnIDs = nil
	invIdx := toAdd.Type == descpb.IndexDescriptor_INVERTED
	for _, colID := range primary.KeyColumnIDs {
		if !presentColIDs.Contains(colID) {
			toAdd.KeySuffixColumnIDs = append(toAdd.KeySuffixColumnIDs, colID)
		} else if invIdx && colID == toAdd.InvertedColumnID() {
			// In an inverted index, the inverted column's value is not equal to the
			// actual data in the row for that column. As a result, if the inverted
			// column happens to also be in the primary key, it's crucial that
			// the index key still be suffixed with that full primary key value to
			// preserve the index semantics.
			// toAdd.KeySuffixColumnIDs = append(toAdd.KeySuffixColumnIDs, colID)
			// However, this functionality is not supported by the execution engine,
			// so prevent it by returning an error.
			col, err := catalog.MustFindColumnByID(table, colID)
			if err != nil {
				return err
			}
			return unimplemented.NewWithIssuef(84405,
				"primary key column %s cannot be present in an inverted index",
				col.GetName(),
			)
		}
	}
	return nil
}

// disallowDroppingPrimaryIndexReferencedInUDFOrView returns an non-nil error
// if current primary index is referenced explicitly in a UDF or view.
// This is used for ADD COLUMN, DROP COLUMN, and ALTER PRIMARY KEY commands
// because their implementation could need to drop the old/current primary index
// and create new ones.
func (p *planner) disallowDroppingPrimaryIndexReferencedInUDFOrView(
	ctx context.Context, tableDesc *tabledesc.Mutable,
) error {
	currentPrimaryIndex := tableDesc.GetPrimaryIndex()
	for _, tableRef := range tableDesc.DependedOnBy {
		if tableRef.IndexID == currentPrimaryIndex.GetID() {
			// canRemoveDependent with `DropDefault` will return the right error.
			err := p.canRemoveDependent(
				ctx, "index", currentPrimaryIndex.GetName(), tableDesc.ParentID, tableRef, tree.DropDefault)
			if err != nil {
				return errors.WithDetail(err, sqlerrors.PrimaryIndexSwapDetail)
			}
		}
	}
	return nil
}
