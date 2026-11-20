// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package redact_test

import (
	"context"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/desctestutils"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/funcdesc"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/redact"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/tabledesc"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/sqlutils"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/stretchr/testify/require"
)

func TestRedactQueries(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	ctx := context.Background()
	srv, db, kvDB := serverutils.StartServer(t, base.TestServerArgs{})
	defer srv.Stopper().Stop(ctx)
	codec := srv.ApplicationLayer().Codec()
	tdb := sqlutils.MakeSQLRunner(db)
	tdb.Exec(t, "CREATE TABLE kv (k INT PRIMARY KEY, v STRING)")
	tdb.Exec(t, "CREATE VIEW view AS SELECT k, v FROM kv WHERE v <> 'constant literal'")
	tdb.Exec(t, "CREATE TABLE ctas AS SELECT k, v FROM kv WHERE v <> 'constant literal'")
	tdb.Exec(t, `
CREATE FUNCTION f1() RETURNS INT
LANGUAGE SQL
AS $$
SELECT k FROM kv WHERE v != 'foo';
SELECT k FROM kv WHERE v = 'bar';
$$`)

	t.Run("view", func(t *testing.T) {
		view := desctestutils.TestingGetTableDescriptor(
			kvDB, codec, "defaultdb", "public", "view",
		)
		mut := tabledesc.NewBuilder(view.TableDesc()).BuildCreatedMutableTable()
		require.Empty(t, redact.Redact(mut.DescriptorProto()))
		require.Equal(t, `SELECT k, v FROM defaultdb.public.kv WHERE v != '_'`, mut.ViewQuery)
	})

	t.Run("create table as", func(t *testing.T) {
		ctas := desctestutils.TestingGetTableDescriptor(
			kvDB, codec, "defaultdb", "public", "ctas",
		)
		mut := tabledesc.NewBuilder(ctas.TableDesc()).BuildCreatedMutableTable()
		require.Empty(t, redact.Redact(mut.DescriptorProto()))
		require.Equal(t, `SELECT k, v FROM defaultdb.public.kv WHERE v != '_'`, mut.CreateQuery)
	})

	t.Run("create function sql", func(t *testing.T) {
		fn := desctestutils.TestingGetFunctionDescriptor(kvDB, codec, "defaultdb", "public", "f1")
		mut := funcdesc.NewBuilder(fn.FuncDesc()).BuildCreatedMutableFunction()
		require.Empty(t, redact.Redact(mut.DescriptorProto()))
		require.Equal(t, `SELECT k FROM defaultdb.public.kv WHERE v != '_'; SELECT k FROM defaultdb.public.kv WHERE v = '_';`, mut.FunctionBody)
	})
}
