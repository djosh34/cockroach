// Copyright 2020 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package catformat

import (
	"context"
	"fmt"
	"strconv"

	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/catpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/schemaexpr"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/sessiondata"
	"github.com/cockroachdb/errors"
)

// IndexDisplayMode influences how an index should be formatted for pretty print
// in IndexForDisplay function.
type IndexDisplayMode int

const (
	// IndexDisplayShowCreate indicates index definition to be printed as a CREATE
	// INDEX statement.
	IndexDisplayShowCreate IndexDisplayMode = iota
	// IndexDisplayDefOnly indicates index definition to be printed as INDEX
	// definition format within a CREATE TABLE statement.
	IndexDisplayDefOnly
)

// IndexForDisplay formats an index descriptor as a SQL string. It converts user
// defined types in partial index predicate expressions to a human-readable
// form.
//
// If tableName is anonymous then no table name is included in the formatted
// string. For example:
//
//	INDEX i (a) WHERE b > 0
//
// If tableName is not anonymous, then "ON" and the name is included:
//
//	INDEX i ON t (a) WHERE b > 0
func IndexForDisplay(
	ctx context.Context,
	table catalog.TableDescriptor,
	tableName *tree.TableName,
	index catalog.Index,
	formatFlags tree.FmtFlags,
	semaCtx *tree.SemaContext,
	sessionData *sessiondata.SessionData,
	displayMode IndexDisplayMode,
) (string, error) {
	return indexForDisplay(
		ctx,
		table,
		tableName,
		index.IndexDesc(),
		index.Primary(),
		formatFlags,
		semaCtx,
		sessionData,
		displayMode,
	)
}

func indexForDisplay(
	ctx context.Context,
	table catalog.TableDescriptor,
	tableName *tree.TableName,
	index *descpb.IndexDescriptor,
	isPrimary bool,
	formatFlags tree.FmtFlags,
	semaCtx *tree.SemaContext,
	sessionData *sessiondata.SessionData,
	displayMode IndexDisplayMode,
) (string, error) {
	// Please also update CreateIndex's "Format" method in
	// pkg/sql/sem/tree/create.go if there's any update to index definition
	// components.
	if displayMode == IndexDisplayShowCreate && *tableName == descpb.AnonymousTable {
		return "", errors.New("tableName must be set for IndexDisplayShowCreate mode")
	}

	f := tree.NewFmtCtx(formatFlags)
	if displayMode == IndexDisplayShowCreate {
		f.WriteString("CREATE ")
	}
	if index.Unique {
		f.WriteString("UNIQUE ")
	}
	if !f.HasFlags(tree.FmtPGCatalog) && index.Type == descpb.IndexDescriptor_INVERTED {
		f.WriteString("INVERTED ")
	}
	f.WriteString("INDEX ")
	f.FormatNameP(&index.Name)
	if *tableName != descpb.AnonymousTable {
		f.WriteString(" ON ")
		f.FormatNode(tableName)
	}

	if f.HasFlags(tree.FmtPGCatalog) {
		f.WriteString(" USING")
		if index.Type == descpb.IndexDescriptor_INVERTED {
			f.WriteString(" gin")
		} else {
			f.WriteString(" btree")
		}
	}

	f.WriteString(" (")
	if err := FormatIndexElements(ctx, table, index, f, semaCtx, sessionData); err != nil {
		return "", err
	}
	f.WriteByte(')')

	if index.IsSharded() {
		if f.HasFlags(tree.FmtPGCatalog) {
			fmt.Fprintf(f, " USING HASH WITH (bucket_count=%v)",
				index.Sharded.ShardBuckets)
		} else {
			f.WriteString(" USING HASH")
		}
	}

	if !isPrimary && len(index.StoreColumnNames) > 0 {
		f.WriteString(" STORING (")
		for i := range index.StoreColumnNames {
			if i > 0 {
				f.WriteString(", ")
			}
			f.FormatNameP(&index.StoreColumnNames[i])
		}
		f.WriteByte(')')
	}

	if !f.HasFlags(tree.FmtPGCatalog) {
		formatStorageConfigs(index, f)
	}

	if index.IsPartial() {
		predFmtFlag := tree.FmtParsable
		if f.HasFlags(tree.FmtPGCatalog) {
			predFmtFlag = tree.FmtPGCatalog
		} else {
			if f.HasFlags(tree.FmtMarkRedactionNode) {
				predFmtFlag |= tree.FmtMarkRedactionNode
			}
			if f.HasFlags(tree.FmtOmitNameRedaction) {
				predFmtFlag |= tree.FmtOmitNameRedaction
			}
		}
		pred, err := schemaexpr.FormatExprForDisplay(ctx, table, index.Predicate, semaCtx, sessionData, predFmtFlag)
		if err != nil {
			return "", err
		}

		f.WriteString(" WHERE ")
		if f.HasFlags(tree.FmtPGCatalog) {
			f.WriteString("(")
			f.WriteString(pred)
			f.WriteString(")")
		} else {
			f.WriteString(pred)
		}
	}

	if idxInvisibility := index.Invisibility; idxInvisibility != 0.0 {
		if idxInvisibility == 1.0 {
			f.WriteString(" NOT VISIBLE")
		} else {
			f.WriteString(" VISIBILITY ")
			f.WriteString(fmt.Sprintf("%.2f", 1-index.Invisibility))
		}
	}

	return f.CloseAndGetString(), nil
}

// FormatIndexElements formats the key columns an index. If the column is an
// inaccessible computed column, the computed column expression is formatted.
// Otherwise, the column name is formatted. Each column is separated by commas
// and includes the direction of the index if the index is not an inverted
// index.
func FormatIndexElements(
	ctx context.Context,
	table catalog.TableDescriptor,
	index *descpb.IndexDescriptor,
	f *tree.FmtCtx,
	semaCtx *tree.SemaContext,
	sessionData *sessiondata.SessionData,
) error {
	elemFmtFlag := tree.FmtParsable
	if f.HasFlags(tree.FmtPGCatalog) {
		elemFmtFlag = tree.FmtPGCatalog
	} else {
		if f.HasFlags(tree.FmtMarkRedactionNode) {
			elemFmtFlag |= tree.FmtMarkRedactionNode
		}
		if f.HasFlags(tree.FmtOmitNameRedaction) {
			elemFmtFlag |= tree.FmtOmitNameRedaction
		}
	}

	startIdx := index.ExplicitColumnStartIdx()
	for i, n := startIdx, len(index.KeyColumnIDs); i < n; i++ {
		col, err := catalog.MustFindColumnByID(table, index.KeyColumnIDs[i])
		if err != nil {
			return err
		}
		if i > startIdx {
			f.WriteString(", ")
		}
		if col.IsExpressionIndexColumn() {
			expr, err := schemaexpr.FormatExprForExpressionIndexDisplay(
				ctx, table, col.GetComputeExpr(), semaCtx, sessionData, elemFmtFlag,
			)
			if err != nil {
				return err
			}
			f.WriteString(expr)
		} else {
			f.FormatNameP(&index.KeyColumnNames[i])
		}
		if index.Type == descpb.IndexDescriptor_INVERTED &&
			col.GetID() == index.InvertedColumnID() && len(index.InvertedColumnKinds) > 0 {
			// The kind of the inverted column is the last one; see
			// InvertedColumnKind.
			switch index.InvertedColumnKinds[len(index.InvertedColumnKinds)-1] {
			case catpb.InvertedIndexColumnKind_TRIGRAM:
				f.WriteString(" gin_trgm_ops")
			}
		}
		// The last column of an inverted index cannot have a DESC direction.
		// Since the default direction is ASC, we omit the direction entirely
		// for inverted index columns.
		if i < n-1 || index.Type != descpb.IndexDescriptor_INVERTED {
			f.WriteByte(' ')
			f.WriteString(index.KeyColumnDirections[i].String())
		}
	}
	return nil
}

// formatStorageConfigs writes the index's storage configurations to the given
// format context.
func formatStorageConfigs(index *descpb.IndexDescriptor, f *tree.FmtCtx) {
	if index.IsSharded() {
		f.WriteString(" WITH (bucket_count=")
		f.WriteString(strconv.FormatInt(int64(index.Sharded.ShardBuckets), 10))
		f.WriteString(")")
	}
}
