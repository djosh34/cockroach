// Copyright 2022 The Cockroach Authors.
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
	"github.com/cockroachdb/cockroach/pkg/kv"
	"github.com/cockroachdb/cockroach/pkg/kv/kvpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/systemschema"
	"github.com/cockroachdb/cockroach/pkg/upgrade"
)

func descIDSequenceForSystemTenant(
	ctx context.Context, _ clusterversion.ClusterVersion, d upgrade.TenantDeps,
) error {
	if !d.Codec.ForSystemTenant() {
		return nil
	}
	return d.DB.KV().Txn(ctx, func(ctx context.Context, txn *kv.Txn) error {
		oldEntry, err := txn.GetForUpdate(ctx, keys.LegacyDescIDGenerator, kvpb.BestEffort)
		if err != nil {
			return err
		}
		id, created, err := CreateSystemTableInTxn(
			ctx, d.Settings, txn, keys.SystemSQLCodec, systemschema.DescIDSequence,
		)
		if err != nil || !created {
			return err
		}
		// Install the appropriate value for the sequence. Note that we use the
		// existence of the sequence above to make this transaction idempotent.
		return txn.Put(ctx, d.Codec.SequenceKey(uint32(id)), oldEntry.ValueInt())
	})
}
