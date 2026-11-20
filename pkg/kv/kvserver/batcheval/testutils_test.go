// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package batcheval_test

import (
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/storage"
	"github.com/cockroachdb/cockroach/pkg/testutils/storageutils"
)

type kvs = storageutils.KVs

var (
	pointKV            = storageutils.PointKV
	pointKVWithLocalTS = storageutils.PointKVWithLocalTS
	rangeKey           = storageutils.RangeKey
	rangeKV            = storageutils.RangeKV
	rangeKVWithLocalTS = storageutils.RangeKVWithLocalTS
	wallTS             = storageutils.WallTS
	scanEngine         = storageutils.ScanEngine
)

type wrappedBatch struct {
	storage.Batch
	clearIterCount int
}

func (wb *wrappedBatch) ClearMVCCIteratorRange(
	start, end roachpb.Key, pointKeys, rangeKeys bool,
) error {
	wb.clearIterCount++
	return wb.Batch.ClearMVCCIteratorRange(start, end, pointKeys, rangeKeys)
}
