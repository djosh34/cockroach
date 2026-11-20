// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package zone

import (
	"github.com/cockroachdb/cockroach/pkg/config/zonepb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
	"github.com/cockroachdb/cockroach/pkg/util/protoutil"
)

// ZoneConfigWithRawBytes wraps a zone config together with its expected
// raw bytes. For cached modified zone configs, the raw bytes should be the
// deserialized bytes from the zone config proto. Though, the raw bytes should
// be the raw value read from kv when it's first read from storage.
type ZoneConfigWithRawBytes struct {
	zc *zonepb.ZoneConfig
	rb []byte
}

// NewZoneConfigWithRawBytes creates a new ZoneConfigWithRawBytes.
func NewZoneConfigWithRawBytes(zc *zonepb.ZoneConfig, rb []byte) *ZoneConfigWithRawBytes {
	return &ZoneConfigWithRawBytes{zc: zc, rb: rb}
}

// Size implements the catalog.ZoneConfig interface. It returns the size of data
// represented by this struct not the actual size of this struct.
func (zc *ZoneConfigWithRawBytes) Size() int {
	return zc.zc.Size() + len(zc.rb)
}

// ZoneConfigProto implements the catalog.ZoneConfig interface. It returns the
// underlying zone config.
func (zc *ZoneConfigWithRawBytes) ZoneConfigProto() *zonepb.ZoneConfig {
	return zc.zc
}

// GetRawBytesInStorage implements the catalog.ZoneConfig interface. It returns
// the raw bytes.
func (zc *ZoneConfigWithRawBytes) GetRawBytesInStorage() []byte {
	return zc.rb
}

// Clone returns a ZoneConfigWithRawBytes with a deep copy of ZoneConfig. It's
// safe to not clone the raw bytes because the cput would fail if raw bytes
// differs from the original bytes.
func (zc *ZoneConfigWithRawBytes) Clone() catalog.ZoneConfig {
	if zc == nil {
		return nil
	}
	clone := protoutil.Clone(zc.zc).(*zonepb.ZoneConfig)
	return NewZoneConfigWithRawBytes(clone, zc.rb)
}
