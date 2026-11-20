// Copyright 2022 The Cockroach Authors.
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

	"github.com/cockroachdb/cockroach/pkg/config"
	"github.com/cockroachdb/cockroach/pkg/config/zonepb"
	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/kv"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/catalogkeys"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descbuilder"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/zone"
)

type systemZoneConfigHelper struct {
	cfg   *config.SystemConfig
	codec keys.SQLCodec
}

// MaybeGetTable implements the catalog.ZoneConfigHydrationHelper interface.
func (h *systemZoneConfigHelper) MaybeGetTable(
	ctx context.Context, txn *kv.Txn, id descpb.ID,
) (catalog.TableDescriptor, error) {
	val := h.cfg.GetValue(catalogkeys.MakeDescMetadataKey(h.codec, id))
	if val == nil {
		return nil, nil
	}

	b, err := descbuilder.FromSerializedValue(val)
	if err != nil {
		return nil, err
	}
	if b == nil || b.DescriptorType() != catalog.Table {
		return nil, nil
	}
	return b.BuildImmutable().(catalog.TableDescriptor), nil
}

// MaybeGetZoneConfig implements the catalog.ZoneConfigHydrationHelper
// interface.
func (h *systemZoneConfigHelper) MaybeGetZoneConfig(
	ctx context.Context, txn *kv.Txn, id descpb.ID,
) (catalog.ZoneConfig, error) {
	val := h.cfg.GetValue(config.MakeZoneKey(h.codec, id))
	if val == nil {
		return nil, nil
	}

	var z zonepb.ZoneConfig
	if err := val.GetProto(&z); err != nil {
		return nil, err
	}
	rawBytes, err := val.GetBytes()
	if err != nil {
		return nil, err
	}
	return zone.NewZoneConfigWithRawBytes(&z, rawBytes), nil
}
