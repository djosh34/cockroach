// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package batcheval

import (
	"testing"

	"github.com/cockroachdb/cockroach/pkg/kv/kvpb"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/stretchr/testify/require"
)

func TestMergeGCRangeBoundaries(t *testing.T) {
	defer leaktest.AfterTest(t)()

	gcr := func(start, end roachpb.Key) kvpb.GCRequest_GCRangeKey {
		return kvpb.GCRequest_GCRangeKey{
			StartKey: start,
			EndKey:   end,
		}
	}
	span := func(start, end roachpb.Key) roachpb.Span {
		return roachpb.Span{
			Key:    start,
			EndKey: end,
		}
	}
	key := func(k string) roachpb.Key {
		return roachpb.Key(k)
	}
	preKey := func(k string) roachpb.Key {
		l, _ := rangeTombstonePeekBounds(key(k), key(k+"zzzzzzz"), nil, nil)
		return l
	}
	postKey := func(k string) roachpb.Key {
		_, r := rangeTombstonePeekBounds(key(""), key(k), nil, nil)
		return r
	}

	for _, d := range []struct {
		name       string
		rangeStart roachpb.Key
		rangeEnd   roachpb.Key
		rangeKeys  []kvpb.GCRequest_GCRangeKey
		spans      []roachpb.Span
	}{
		{
			name:       "empty",
			rangeStart: key("a"),
			rangeEnd:   key("b"),
			rangeKeys:  []kvpb.GCRequest_GCRangeKey{},
			spans:      nil,
		},
		{
			name:       "full range",
			rangeStart: key("a"),
			rangeEnd:   key("b"),
			rangeKeys: []kvpb.GCRequest_GCRangeKey{
				gcr(key("a"), key("b")),
			},
			spans: []roachpb.Span{
				span(key("a"), key("b")),
			},
		},
		{
			name:       "sub range",
			rangeStart: key("a"),
			rangeEnd:   key("z"),
			rangeKeys: []kvpb.GCRequest_GCRangeKey{
				gcr(key("c"), key("d")),
			},
			spans: []roachpb.Span{
				span(preKey("c"), postKey("d")),
			},
		},
		{
			name:       "non adjacent",
			rangeStart: key("a"),
			rangeEnd:   key("z"),
			rangeKeys: []kvpb.GCRequest_GCRangeKey{
				gcr(key("c"), key("d")),
				gcr(key("e"), key("f")),
			},
			spans: []roachpb.Span{
				span(preKey("c"), postKey("d")),
				span(preKey("e"), postKey("f")),
			},
		},
		{
			name:       "merge adjacent",
			rangeStart: key("a"),
			rangeEnd:   key("z"),
			rangeKeys: []kvpb.GCRequest_GCRangeKey{
				gcr(key("a"), key("b")),
				gcr(key("b"), key("c")),
				gcr(key("c"), key("d")),
			},
			spans: []roachpb.Span{
				span(key("a"), postKey("d")),
			},
		},
	} {
		t.Run(d.name, func(t *testing.T) {
			spans := makeLookupBoundariesForGCRanges(d.rangeStart, d.rangeEnd, d.rangeKeys)
			merged := mergeAdjacentSpans(spans)
			require.Equal(t, d.spans, merged, "combined spans")
		})
	}
}
