// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package fetchpb

import (
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/catenumpb"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
	"github.com/cockroachdb/cockroach/pkg/util/encoding"
)

// TODO(yuzefovich): consider moving this package somewhere close to rowenc
// given that effectively using this package in roachpb is saying that rows and
// column encoding is a shared concept between sql and KV. Furthermore, we could
// probably make keyside and valueside not depend on sql/catalog and then pull
// them out of the sql tree altogether.

// IndexFetchSpecVersionInitial is the initial IndexFetchSpec version.
const IndexFetchSpecVersionInitial = 1

// KeyColumns returns the key columns in the index, excluding any key suffix
// columns.
func (s *IndexFetchSpec) KeyColumns() []IndexFetchSpec_KeyColumn {
	return s.KeyAndSuffixColumns[:len(s.KeyAndSuffixColumns)-int(s.NumKeySuffixColumns)]
}

// KeyFullColumns returns the key columns in the index, plus all key suffix
// columns if that index is not a unique index. It parallels
// TableDescriptor.IndexFullColumns.
func (s *IndexFetchSpec) KeyFullColumns() []IndexFetchSpec_KeyColumn {
	if s.IsUniqueIndex {
		// For unique indexes, the suffix columns are not part of the key (except
		// when the key columns contain a NULL).
		return s.KeyColumns()
	}
	return s.KeyAndSuffixColumns
}

// KeySuffixColumns returns the key suffix columns.
func (s *IndexFetchSpec) KeySuffixColumns() []IndexFetchSpec_KeyColumn {
	return s.KeyAndSuffixColumns[len(s.KeyAndSuffixColumns)-int(s.NumKeySuffixColumns):]
}

// FetchedColumnTypes returns the types of the fetched columns in a slice.
func (s *IndexFetchSpec) FetchedColumnTypes() []*types.T {
	res := make([]*types.T, len(s.FetchedColumns))
	for i := range res {
		res[i] = s.FetchedColumns[i].Type
	}
	return res
}

// DatumEncoding returns the datum encoding that corresponds to the key column
// direction.
func (c *IndexFetchSpec_KeyColumn) DatumEncoding() catenumpb.DatumEncoding {
	if c.Direction == catenumpb.IndexColumn_DESC {
		return catenumpb.DatumEncoding_DESCENDING_KEY
	}
	return catenumpb.DatumEncoding_ASCENDING_KEY
}

// EncodingDirection returns the encoding direction for the key column.
func (c *IndexFetchSpec_KeyColumn) EncodingDirection() encoding.Direction {
	if c.Direction == catenumpb.IndexColumn_DESC {
		return encoding.Descending
	}
	return encoding.Ascending
}
