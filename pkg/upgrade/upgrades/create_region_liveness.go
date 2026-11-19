// Copyright 2023 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package upgrades

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/clusterversion"
	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descs"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/systemschema"
	"github.com/cockroachdb/cockroach/pkg/upgrade"
)

// createRegionLivenessTables creates the system.region_liveness table.
func createRegionLivenessTables(
	ctx context.Context, cs clusterversion.ClusterVersion, d upgrade.TenantDeps,
) error {
	// Since this is a re-use of an old key space, invalid data might exists,
	// so lets clear it out first.
	if err := d.DB.DescsTxn(ctx, func(ctx context.Context, txn descs.Txn) error {
		tablePrefix := d.Codec.TablePrefix(keys.RegionLivenessTableID)
		_, err := txn.KV().DelRange(ctx, tablePrefix, tablePrefix.PrefixEnd(), false)
		return err
	}); err != nil {
		return err
	}

	if err := createSystemTable(ctx, d.DB, d.Settings, d.Codec,
		systemschema.RegionLivenessTable); err != nil {
		return err
	}

	return bumpSystemDatabaseSchemaVersion(ctx, cs, d)
}
