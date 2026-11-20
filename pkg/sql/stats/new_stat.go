// Copyright 2018 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package stats

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/clusterversion"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/isql"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
	"github.com/cockroachdb/cockroach/pkg/util/protoutil"
	"github.com/cockroachdb/errors"
)

// InsertNewStats inserts a slice of statistics at the current time into the
// system table.
func InsertNewStats(
	ctx context.Context, settings *cluster.Settings, txn isql.Txn, tableStats []*TableStatisticProto,
) error {
	var err error
	for _, statistic := range tableStats {
		err = InsertNewStat(
			ctx,
			settings,
			txn,
			statistic.TableID,
			statistic.Name,
			statistic.ColumnIDs,
			int64(statistic.RowCount),
			int64(statistic.DistinctCount),
			int64(statistic.NullCount),
			int64(statistic.AvgSize),
			statistic.HistogramData,
			statistic.PartialPredicate,
			statistic.FullStatisticID,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// InsertNewStat inserts a new statistic in the system table.
//
// The stats cache will automatically update asynchronously (as well as the
// stats caches on all other nodes).
func InsertNewStat(
	ctx context.Context,
	settings *cluster.Settings,
	txn isql.Txn,
	tableID descpb.ID,
	name string,
	columnIDs []descpb.ColumnID,
	rowCount, distinctCount, nullCount, avgSize int64,
	h *HistogramData,
	partialPredicate string,
	fullStatisticID uint64,
) error {
	// We must pass a nil interface{} if we want to insert a NULL.
	var nameVal, histogramVal interface{}
	if name != "" {
		nameVal = name
	}
	if h != nil {
		var err error
		histogramVal, err = protoutil.Marshal(h)
		if err != nil {
			return err
		}
	}

	columnIDsVal := tree.NewDArray(types.Int)
	for _, c := range columnIDs {
		if err := columnIDsVal.Append(tree.NewDInt(tree.DInt(int(c)))); err != nil {
			return err
		}
	}

	if !settings.Version.IsActive(ctx, clusterversion.V23_1AddPartialStatisticsColumns) {
		if partialPredicate != "" {
			return errors.New("unable to insert new partial statistic as cluster version is from before V23.1.")
		}
		_, err := txn.Exec(
			ctx, "insert-statistic", txn.KV(),
			`INSERT INTO system.table_statistics (
					"tableID",
					"name",
					"columnIDs",
					"rowCount",
					"distinctCount",
					"nullCount",
					"avgSize",
					histogram
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			tableID,
			nameVal,
			columnIDsVal,
			rowCount,
			distinctCount,
			nullCount,
			avgSize,
			histogramVal,
		)
		return err
	}

	// Need to assign to a nil interface{} to be able
	// to insert NULL value.
	var predicateValue interface{}
	if partialPredicate != "" {
		predicateValue = partialPredicate
	}

	_, err := txn.Exec(
		ctx, "insert-statistic", txn.KV(),
		`INSERT INTO system.table_statistics (
					"tableID",
					"name",
					"columnIDs",
					"rowCount",
					"distinctCount",
					"nullCount",
					"avgSize",
					histogram,
					"partialPredicate",
					"fullStatisticID"
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		tableID,
		nameVal,
		columnIDsVal,
		rowCount,
		distinctCount,
		nullCount,
		avgSize,
		histogramVal,
		predicateValue,
		fullStatisticID,
	)
	return err
}
