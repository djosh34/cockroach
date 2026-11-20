// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package gcjob

import (
	"context"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/kv"
	"github.com/cockroachdb/cockroach/pkg/kv/kvclient/kvcoord"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/stretchr/testify/require"
)

// TestDeleteAllSpanData checks that deleteAllSpanData properly deletes data
// from the provided span.
func TestDeleteAllSpanData(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	ctx := context.Background()
	srv, _, kvDB := serverutils.StartServer(t, base.TestServerArgs{})
	defer srv.Stopper().Stop(ctx)
	s := srv.ApplicationLayer()

	getKey := func(key string) roachpb.Key {
		return append(s.Codec().TenantPrefix(), key...)
	}

	b := &kv.Batch{}
	b.Put(getKey("aa"), "1")
	b.Put(getKey("ab"), "2")
	b.Put(getKey("bb"), "3")
	require.NoError(t, kvDB.Run(context.Background(), b))

	deleteSpan := roachpb.RSpan{Key: roachpb.RKey(getKey("aa")), EndKey: roachpb.RKey(getKey("bb"))}
	distSender := s.DistSenderI().(*kvcoord.DistSender)
	require.NoError(t, deleteAllSpanData(ctx, kvDB, distSender, deleteSpan))

	kvs, err := kvDB.Scan(ctx, getKey("aa"), getKey("bb"), 0)
	require.NoError(t, err)
	require.Equal(t, 0, len(kvs))
}
