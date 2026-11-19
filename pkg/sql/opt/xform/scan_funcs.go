// Copyright 2020 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package xform

import (
	"github.com/cockroachdb/cockroach/pkg/sql/opt"
	"github.com/cockroachdb/cockroach/pkg/sql/opt/cat"
	"github.com/cockroachdb/cockroach/pkg/sql/opt/memo"
	"github.com/cockroachdb/cockroach/pkg/sql/opt/props/physical"
	"github.com/cockroachdb/errors"
)

// GenerateIndexScans enumerates all non-inverted secondary indexes on the given
// Scan operator's table and generates an alternate Scan operator for each index
// that includes the set of needed columns specified in the ScanOpDef.
//
// This transformation can only consider partial indexes that are guaranteed to
// index every row in the table. Therefore, only partial indexes with predicates
// that always evaluate to true are considered. Such an index is pseudo-partial
// in that it behaves the exactly the same as a non-partial secondary index.
//
// NOTE: This does not generate index joins for non-covering indexes (except in
// case of ForceIndex). Index joins are usually only introduced "one level up",
// when the Scan operator is wrapped by an operator that constrains or limits
// scan output in some way (e.g. Select, Limit, InnerJoin). Index joins are only
// lower cost when their input does not include all rows from the table. See
// GenerateConstrainedScans, GenerateLimitedScans, and
// GenerateLimitedGroupByScans for cases where index joins are introduced into
// the memo.
func (c *CustomFuncs) GenerateIndexScans(
	grp memo.RelExpr, required *physical.Required, scanPrivate *memo.ScanPrivate,
) {
	// Iterate over all non-inverted and non-partial secondary indexes.
	var pkCols opt.ColSet
	var iter scanIndexIter
	iter.Init(c.e.evalCtx, c.e, c.e.mem, &c.im, scanPrivate, nil /* filters */, rejectPrimaryIndex|rejectInvertedIndexes)
	iter.ForEach(func(index cat.Index, filters memo.FiltersExpr, indexCols opt.ColSet, isCovering bool, constProj memo.ProjectionsExpr) {
		// The iterator only produces pseudo-partial indexes (the predicate is
		// true) because no filters are passed to iter.Init to imply a partial
		// index predicate. constProj is a projection of constant values based
		// on a partial index predicate. It should always be empty because a
		// pseudo-partial index cannot hold a column constant. If it is not, we
		// panic to avoid performing a logically incorrect transformation.
		if len(constProj) != 0 {
			panic(errors.AssertionFailedf("expected constProj to be empty"))
		}

		// If the secondary index includes the set of needed columns, then construct
		// a new Scan operator using that index.
		if isCovering {
			scan := memo.ScanExpr{ScanPrivate: *scanPrivate}
			scan.Index = index.Ordinal()
			md := c.e.mem.Metadata()
			tabMeta := md.TableMeta(scan.Table)
			scan.Distribution.FromIndexScan(tabMeta, scan.Index)
			c.e.mem.AddScanToGroup(&scan, grp)
			return
		}

		// Otherwise, if the index must be forced, then construct an IndexJoin
		// operator that provides the columns missing from the index. Note that
		// if ForceIndex=true, scanIndexIter only returns the one index that is
		// being forced, so no need to check that here.
		// AllowUnconstrainedNonCoveringIndexScan allows all index access paths to
		// be explored, even when non-covering and unconstrained, without forcing a
		// particular index.
		if !scanPrivate.Flags.ForceIndex && !c.e.mem.AllowUnconstrainedNonCoveringIndexScan() {
			return
		}

		var sb indexScanBuilder
		sb.Init(c, scanPrivate.Table)

		// Calculate the PK columns once.
		if pkCols.Empty() {
			pkCols = c.PrimaryKeyCols(scanPrivate.Table)
		}

		// Scan whatever columns we need which are available from the index, plus
		// the PK columns.
		newScanPrivate := *scanPrivate
		newScanPrivate.Distribution.Regions = nil
		newScanPrivate.Index = index.Ordinal()
		newScanPrivate.Cols = indexCols.Intersection(scanPrivate.Cols)
		newScanPrivate.Cols.UnionWith(pkCols)
		sb.SetScan(&newScanPrivate)

		sb.AddIndexJoin(scanPrivate.Cols)
		sb.Build(grp)
	})
}

// ScanPrivateCols returns the ColSet of a ScanPrivate.
func (c *CustomFuncs) ScanPrivateCols(sp *memo.ScanPrivate) opt.ColSet {
	return sp.Cols
}
