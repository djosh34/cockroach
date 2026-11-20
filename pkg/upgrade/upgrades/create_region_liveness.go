// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

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
