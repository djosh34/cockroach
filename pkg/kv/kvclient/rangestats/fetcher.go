// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

// Package rangestats contains code to fetch range stats.
package rangestats

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/kv"
	"github.com/cockroachdb/cockroach/pkg/kv/kvpb"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/eval"
)

// Fetcher implements eval.RangeStatsFetcher.
type Fetcher struct {
	db *kv.DB
}

var _ eval.RangeStatsFetcher = (*Fetcher)(nil)

// NewFetcher constructs a new Fetcher.
func NewFetcher(db *kv.DB) *Fetcher {
	return &Fetcher{db: db}
}

// RangeStats is part of the eval.RangeStatsFetcher interface.
func (f Fetcher) RangeStats(
	ctx context.Context, keys ...roachpb.Key,
) ([]*kvpb.RangeStatsResponse, error) {
	var ba kv.Batch
	reqs := make([]kvpb.RangeStatsRequest, len(keys))
	for i, k := range keys {
		reqs[i].Key = k
		ba.AddRawRequest(&reqs[i])
	}
	if err := f.db.Run(ctx, &ba); err != nil {
		return nil, err
	}
	resps := make([]*kvpb.RangeStatsResponse, len(keys))
	for i, r := range ba.RawResponse().Responses {
		resps[i] = r.GetInner().(*kvpb.RangeStatsResponse)
	}
	return resps, nil
}
