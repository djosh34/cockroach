// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package scbuildstmt

import (
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/catpb"
	"github.com/cockroachdb/cockroach/pkg/sql/pgwire/pgcode"
	"github.com/cockroachdb/cockroach/pkg/sql/pgwire/pgerror"
	"github.com/cockroachdb/cockroach/pkg/sql/schemachanger/scpb"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
)

func alterTableSetNotNull(
	b BuildCtx, tn *tree.TableName, tbl *scpb.Table, t *tree.AlterTableSetNotNull,
) {
	alterColumnPreChecks(b, tn, tbl, t.Column)
	columnID := getColumnIDFromColumnName(b, tbl.TableID, t.Column, true /*required */)
	if isColNotNull(b, tbl.TableID, columnID) {
		return
	}
	// Block alters on system columns.
	scpb.ForEachColumn(
		b.QueryByID(tbl.TableID),
		func(_ scpb.Status, _ scpb.TargetStatus, e *scpb.Column) {
			if e.ColumnID == columnID {
				// Block drops on system columns.
				panicIfSystemColumn(e, t.Column.String())
			}
		})
	b.Add(&scpb.ColumnNotNull{
		TableID:  tbl.TableID,
		ColumnID: columnID,
	})
}

// alterColumnPreChecks contains prerequisite checks for all ALTER COLUMN commands.
// All types of ALTER COLUMN commands should call this function first before
// any of its custom logic.
func alterColumnPreChecks(b BuildCtx, tn *tree.TableName, tbl *scpb.Table, columnName tree.Name) {
	// Cannot alter the TTL expiration column with active expiration column.
	scpb.ForEachRowLevelTTL(b.QueryByID(tbl.TableID), func(
		_ scpb.Status, _ scpb.TargetStatus, e *scpb.RowLevelTTL,
	) {
		if columnName == catpb.TTLDefaultExpirationColumnName && e.HasDurationExpr() {
			panic(pgerror.Newf(
				pgcode.InvalidTableDefinition,
				`cannot alter column %s while ttl_expire_after is set`,
				columnName,
			))
		}
	})
}
