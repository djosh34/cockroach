// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package catalog

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/config/zonepb"
	"github.com/cockroachdb/cockroach/pkg/kv"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
)

// ZoneConfig encapsulates zone config and raw bytes read from storage.
type ZoneConfig interface {
	// ZoneConfigProto returns the zone config proto buf struct.
	ZoneConfigProto() *zonepb.ZoneConfig

	// GetRawBytesInStorage returns the corresponding raw bytes read from storage.
	GetRawBytesInStorage() []byte

	// Size returns the underlying bytes used by the instance.
	Size() int

	// Clone returns a deep copy of the object.
	Clone() ZoneConfig
}

// ZoneConfigHydrationHelper is a helper interface for zone config hydration.
type ZoneConfigHydrationHelper interface {
	// MaybeGetTable either return a table descriptor if exists or a nil if not
	// exists.
	MaybeGetTable(ctx context.Context, txn *kv.Txn, id descpb.ID) (TableDescriptor, error)
	// MaybeGetZoneConfig either return a zone config if exists or a nil if not
	// exists.
	MaybeGetZoneConfig(ctx context.Context, txn *kv.Txn, id descpb.ID) (ZoneConfig, error)
}
