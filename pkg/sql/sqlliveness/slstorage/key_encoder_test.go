// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package slstorage

import (
	"bytes"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/sql/enum"
	"github.com/cockroachdb/cockroach/pkg/sql/sqlliveness"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/uuid"
	"github.com/stretchr/testify/require"
)

func TestKeyEncoder(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	sqlCodec := keys.MakeSQLCodec(roachpb.MustMakeTenantID(1337))
	codec := &rbrEncoder{sqlCodec.IndexPrefix(42, 2)}

	t.Run("Prefix", func(t *testing.T) {
		prefix := codec.indexPrefix()

		rem, tenant, err := keys.DecodeTenantPrefix(prefix)
		require.NoError(t, err)
		require.Equal(t, tenant, roachpb.MustMakeTenantID(1337))

		rem, tableID, indexID, err := keys.DecodeTableIDIndexID(rem)
		require.NoError(t, err)
		require.Equal(t, tableID, uint32(42))
		require.Len(t, rem, 0)

		require.Equal(t, indexID, uint32(2))
	})

	t.Run("RoundTrip", func(t *testing.T) {
		id, err := MakeSessionID(enum.One, uuid.MakeV4())
		require.NoError(t, err)

		key, err := codec.encode(id)
		require.NoError(t, err)
		require.True(t, bytes.HasPrefix(key, codec.indexPrefix()))

		decodedID, err := codec.decode(key)
		require.NoError(t, err)
		require.Equal(t, id, decodedID)
	})

	t.Run("EncodeLegacySession", func(t *testing.T) {
		id := sqlliveness.SessionID(uuid.MakeV4().GetBytes())

		key, err := codec.encode(id)
		require.NoError(t, err)
		decodedID, err := codec.decode(key)
		require.NoError(t, err)
		require.Equal(t, id, decodedID)
	})
}
