// Copyright 2016 The Cockroach Authors.
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
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/gossip"
	"github.com/cockroachdb/cockroach/pkg/keys"
	"github.com/cockroachdb/cockroach/pkg/kv"
	"github.com/cockroachdb/cockroach/pkg/kv/kvclient/kvcoord"
	"github.com/cockroachdb/cockroach/pkg/kv/kvclient/rangecache"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/rpc"
	"github.com/cockroachdb/cockroach/pkg/settings/cluster"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/desctestutils"
	"github.com/cockroachdb/cockroach/pkg/sql/catalog/tabledesc"
	"github.com/cockroachdb/cockroach/pkg/sql/distsql"
	"github.com/cockroachdb/cockroach/pkg/sql/execinfra"
	"github.com/cockroachdb/cockroach/pkg/sql/execinfrapb"
	"github.com/cockroachdb/cockroach/pkg/sql/physicalplan"
	"github.com/cockroachdb/cockroach/pkg/sql/physicalplan/replicaoracle"
	"github.com/cockroachdb/cockroach/pkg/sql/randgen"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/eval"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/sessiondata"
	"github.com/cockroachdb/cockroach/pkg/sql/sessiondatapb"
	"github.com/cockroachdb/cockroach/pkg/sql/sqlinstance"
	"github.com/cockroachdb/cockroach/pkg/testutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/skip"
	"github.com/cockroachdb/cockroach/pkg/testutils/sqlutils"
	"github.com/cockroachdb/cockroach/pkg/util"
	"github.com/cockroachdb/cockroach/pkg/util/hlc"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/metric"
	"github.com/cockroachdb/cockroach/pkg/util/randutil"
	"github.com/cockroachdb/cockroach/pkg/util/stop"
	"github.com/cockroachdb/cockroach/pkg/util/tracing"
	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/require"
)

// TestPlanningDuringSplits verifies that table reader planning (resolving
// spans) tolerates concurrent splits and merges.
func TestPlanningDuringSplitsAndMerges(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	const n = 100

	// NB: this test uses StartCluster because it depends on some
	// cluster setting initializations that only testcluster does.
	const numNodes = 1
	tc := serverutils.StartCluster(t, numNodes, base.TestClusterArgs{
		ServerArgs: base.TestServerArgs{UseDatabase: "test"},
	})
	defer tc.Stopper().Stop(context.Background())

	s := tc.Server(0)
	ts := s.ApplicationLayer()
	cdb := ts.DB()
	db := ts.SQLConn(t, serverutils.DBName("test"))

	sqlutils.CreateTable(
		t, db, "t", "x INT PRIMARY KEY, xsquared INT",
		n,
		sqlutils.ToRowFn(sqlutils.RowIdxFn, func(row int) tree.Datum {
			return tree.NewDInt(tree.DInt(row * row))
		}),
	)

	// Start a worker that continuously performs splits in the background.
	stopSplitter := make(chan struct{})
	splitterDone := make(chan struct{})
	go func() {
		defer t.Logf("splitter done")
		defer close(splitterDone)
		rng, _ := randutil.NewTestRand()
		for {
			select {
			case <-stopSplitter:
				return
			default:
			}

			// Split the table at a random row.
			tableDesc := desctestutils.TestingGetTableDescriptor(
				cdb, ts.Codec(), "test", "public", "t",
			)

			val := rng.Intn(n)
			t.Logf("splitting at %d", val)
			pik, err := randgen.TestingMakePrimaryIndexKey(tableDesc, val)
			if err != nil {
				panic(err)
			}

			if _, _, err := s.SplitRange(pik); err != nil {
				panic(err)
			}
		}
	}()
	defer func() {
		close(stopSplitter)
		t.Logf("waiting for splitter to finish")
		<-splitterDone
		t.Logf("done waiting on splitter to finish")
	}()

	sumX, sumXSquared := 0, 0
	for x := 1; x <= n; x++ {
		sumX += x
		sumXSquared += x * x
	}

	// Run queries continuously in parallel workers. We need more than one worker
	// because some queries result in cache updates, and we want to verify
	// race conditions when planning during cache updates (see #15249).
	const numQueriers = 4

	var wg sync.WaitGroup
	wg.Add(numQueriers)

	for i := 0; i < numQueriers; i++ {
		go func(idx int) {
			defer wg.Done()

			// Create a gosql.DB for this worker.
			goDB := ts.SQLConn(t, serverutils.DBName("test"))

			// Limit to 1 connection because we set a session variable.
			goDB.SetMaxOpenConns(1)
			if _, err := goDB.Exec("SET DISTSQL = ALWAYS"); err != nil {
				t.Error(err)
				return
			}

			for run := 0; run < 20; run++ {
				t.Logf("querier %d run %d", idx, run)
				rows, err := goDB.Query("SELECT sum(x), sum(xsquared) FROM t")
				if err != nil {
					t.Error(err)
					return
				}
				if !rows.Next() {
					t.Errorf("no rows")
					return
				}
				var sum, sumSq int
				if err := rows.Scan(&sum, &sumSq); err != nil {
					t.Error(err)
					return
				}
				if sum != sumX || sumXSquared != sumSq {
					t.Errorf("invalid results: expected %d, %d got %d, %d", sumX, sumXSquared, sum, sumSq)
					return
				}
				if rows.Next() {
					t.Error("more than one row")
					return
				}
				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}
	wg.Wait()
	t.Logf("queriers done")
}

// Test that DistSQLReceiver uses inbound metadata to update the
// RangeDescriptorCache.
func TestDistSQLReceiverUpdatesCaches(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)
	ctx := context.Background()

	size := func() int64 { return 2 << 10 }
	st := cluster.MakeTestingClusterSettings()
	stopper := stop.NewStopper()
	defer stopper.Stop(ctx)
	rangeCache := rangecache.NewRangeCache(st, nil /* db */, size, stopper)
	r := MakeDistSQLReceiver(
		ctx,
		&errOnlyResultWriter{}, /* resultWriter */
		tree.Rows,
		rangeCache,
		nil, /* txn */
		nil, /* clockUpdater */
		&SessionTracing{},
	)

	replicas := []roachpb.ReplicaDescriptor{{ReplicaID: 1}, {ReplicaID: 2}, {ReplicaID: 3}}

	descs := []roachpb.RangeDescriptor{
		{RangeID: 1, StartKey: roachpb.RKey("a"), EndKey: roachpb.RKey("c"), InternalReplicas: replicas},
		{RangeID: 2, StartKey: roachpb.RKey("c"), EndKey: roachpb.RKey("e"), InternalReplicas: replicas},
		{RangeID: 3, StartKey: roachpb.RKey("g"), EndKey: roachpb.RKey("z"), InternalReplicas: replicas},
	}

	// Push some metadata and check that the caches are updated with it.
	status := r.Push(nil /* row */, &execinfrapb.ProducerMetadata{
		Ranges: []roachpb.RangeInfo{
			{
				Desc: descs[0],
				Lease: roachpb.Lease{
					Replica:  roachpb.ReplicaDescriptor{NodeID: 1, StoreID: 1, ReplicaID: 1},
					Start:    hlc.MinClockTimestamp,
					Sequence: 1,
				},
			},
			{
				Desc: descs[1],
				Lease: roachpb.Lease{
					Replica:  roachpb.ReplicaDescriptor{NodeID: 2, StoreID: 2, ReplicaID: 2},
					Start:    hlc.MinClockTimestamp,
					Sequence: 1,
				},
			},
		}})
	if status != execinfra.NeedMoreRows {
		t.Fatalf("expected status NeedMoreRows, got: %d", status)
	}
	status = r.Push(nil /* row */, &execinfrapb.ProducerMetadata{
		Ranges: []roachpb.RangeInfo{
			{
				Desc: descs[2],
				Lease: roachpb.Lease{
					Replica:  roachpb.ReplicaDescriptor{NodeID: 3, StoreID: 3, ReplicaID: 3},
					Start:    hlc.MinClockTimestamp,
					Sequence: 1,
				},
			},
		}})
	if status != execinfra.NeedMoreRows {
		t.Fatalf("expected status NeedMoreRows, got: %d", status)
	}

	for i := range descs {
		ri := rangeCache.GetCached(ctx, descs[i].StartKey, false /* inclusive */)
		require.NotNilf(t, ri, "failed to find range for key: %s", descs[i].StartKey)
		require.Equal(t, &descs[i], ri.Desc())
		require.NotNil(t, ri.Lease())
	}
}

// Test that a gateway improves the physical plans that it generates as a result
// of running a badly-planned query and receiving range information in response;
// this range information is used to update caches on the gateway.
func TestDistSQLRangeCachesIntegrationTest(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	// We're going to setup a cluster with 4 nodes. The last one will not be a
	// target of any replication so that its caches stay virgin.

	tc := serverutils.StartCluster(t, 4, /* numNodes */
		base.TestClusterArgs{
			ReplicationMode: base.ReplicationManual,
			ServerArgs: base.TestServerArgs{
				UseDatabase: "test",
			},
		})
	defer tc.Stopper().Stop(context.Background())

	db0 := tc.ServerConn(0)
	sqlutils.CreateTable(t, db0, "left",
		"num INT PRIMARY KEY",
		3, /* numRows */
		sqlutils.ToRowFn(sqlutils.RowIdxFn))
	sqlutils.CreateTable(t, db0, "right",
		"num INT PRIMARY KEY",
		3, /* numRows */
		sqlutils.ToRowFn(sqlutils.RowIdxFn))

	// Disable eviction of the first range from the range cache on node 4 because
	// the unpredictable nature of those updates interferes with the expectations
	// of this test below.
	//
	// TODO(andrei): This is super hacky. What this test really wants to do is to
	// precisely control the contents of the range cache on node 4.
	tc.Server(3).DistSenderI().(*kvcoord.DistSender).DisableFirstRangeUpdates()
	db3 := tc.ServerConn(3)
	// Force the DistSQL on this connection.
	_, err := db3.Exec(`SET CLUSTER SETTING sql.defaults.distsql = always;`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db3.Exec(`SET distsql = always`)
	if err != nil {
		t.Fatal(err)
	}
	// Do a query on node 4 so that it populates the its cache with an initial
	// descriptor containing all the SQL key space. If we don't do this, the state
	// of the cache is left at the whim of gossiping the first descriptor done
	// during cluster startup - it can happen that the cache remains empty, which
	// is not what this test wants.
	_, err = db3.Exec(`SELECT * FROM "left"`)
	if err != nil {
		t.Fatal(err)
	}

	// We're going to split one of the tables, but node 4 is unaware of this.
	_, err = db0.Exec(fmt.Sprintf(`
	ALTER TABLE "right" SPLIT AT VALUES (1), (2), (3);
	ALTER TABLE "right" EXPERIMENTAL_RELOCATE VALUES (ARRAY[%d], 1), (ARRAY[%d], 2), (ARRAY[%d], 3);
	`,
		tc.Server(1).GetFirstStoreID(),
		tc.Server(0).GetFirstStoreID(),
		tc.Server(2).GetFirstStoreID()))
	if err != nil {
		t.Fatal(err)
	}

	// Ensure that the range cache is populated (see #31235).
	_, err = db0.Exec(`SHOW RANGES FROM TABLE "right"`)
	if err != nil {
		t.Fatal(err)
	}

	// Run everything in a transaction, so we're bound on a connection on which we
	// force DistSQL.
	txn, err := db3.BeginTx(context.Background(), nil /* opts */)
	if err != nil {
		t.Fatal(err)
	}

	// Check that the initial planning is suboptimal: the cache on db3 is unaware
	// of the splits and still holds the state after the first dummy query at the
	// beginning of the test, which had everything on the first node.
	query := `SELECT count(1) FROM "left" INNER JOIN "right" USING (num)`
	row := db3.QueryRow(fmt.Sprintf(`EXPLAIN (DISTSQL, JSON) %v`, query))
	var json string
	if err := row.Scan(&json); err != nil {
		t.Fatal(err)
	}
	exp := `"nodeNames":["1","4"]`
	if !strings.Contains(json, exp) {
		t.Fatalf("expected json to contain %s, but json is: %s", exp, json)
	}

	// Run a non-trivial query to force the "wrong range" metadata to flow through
	// a number of components.
	row = txn.QueryRowContext(context.Background(), query)
	var cnt int
	if err := row.Scan(&cnt); err != nil {
		t.Fatal(err)
	}
	if cnt != 3 {
		t.Fatalf("expected 3, got: %d", cnt)
	}
	if err := txn.Rollback(); err != nil {
		t.Fatal(err)
	}

	// Now assert that new plans correctly contain all the nodes. This is expected
	// to be a result of the caches having been updated on the gateway by the
	// previous query.
	row = db3.QueryRow(fmt.Sprintf(`EXPLAIN (DISTSQL, JSON) %v`, query))
	if err := row.Scan(&json); err != nil {
		t.Fatal(err)
	}
	exp = `"nodeNames":["1","2","3","4"]`
	if !strings.Contains(json, exp) {
		t.Fatalf("expected json to contain %s, but json is: %s", exp, json)
	}
}

func TestDistSQLDeadHosts(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	skip.UnderShort(t, "takes 20s")

	const n = 100
	const numNodes = 5

	tc := serverutils.StartCluster(t, numNodes, base.TestClusterArgs{
		ReplicationMode: base.ReplicationManual,
		ServerArgs:      base.TestServerArgs{UseDatabase: "test"},
	})
	defer tc.Stopper().Stop(context.Background())

	db := tc.ServerConn(0)
	db.SetMaxOpenConns(1)
	r := sqlutils.MakeSQLRunner(db)
	r.Exec(t, "CREATE DATABASE test")

	r.Exec(t, "CREATE TABLE t (x INT PRIMARY KEY, xsquared INT)")

	for i := 0; i < numNodes; i++ {
		r.Exec(t, fmt.Sprintf("ALTER TABLE t SPLIT AT VALUES (%d)", n*i/5))
	}

	// Evenly spread the ranges between the first 4 nodes. Only the last range
	// has a replica on the fifth node.
	for i := 0; i < numNodes; i++ {
		r.Exec(t, fmt.Sprintf(
			"ALTER TABLE t EXPERIMENTAL_RELOCATE VALUES (ARRAY[%d,%d,%d], %d)",
			i+1, (i+1)%4+1, (i+2)%4+1, n*i/5,
		))
	}
	r.CheckQueryResults(t,
		`SELECT IF(substring(start_key for 1)='…',start_key,NULL),
            IF(substring(end_key for 1)='…',end_key,NULL),
            lease_holder, replicas FROM [SHOW RANGES FROM TABLE t WITH DETAILS]`,
		[][]string{
			{"NULL", "…/1/0", "1", "{1}"},
			{"…/1/0", "…/1/20", "1", "{1,2,3}"},
			{"…/1/20", "…/1/40", "2", "{2,3,4}"},
			{"…/1/40", "…/1/60", "3", "{1,3,4}"},
			{"…/1/60", "…/1/80", "4", "{1,2,4}"},
			{"…/1/80", "NULL", "5", "{2,3,5}"},
		},
	)

	r.Exec(t, fmt.Sprintf("INSERT INTO t SELECT i, i*i FROM generate_series(1, %d) AS g(i)", n))

	r.Exec(t, "SET DISTSQL = ON")

	// Run a query that uses the entire table and is easy to verify.
	runQuery := func() error {
		log.Infof(context.Background(), "running test query")
		var res int
		if err := db.QueryRow("SELECT sum(xsquared) FROM t").Scan(&res); err != nil {
			return err
		}
		if exp := (n * (n + 1) * (2*n + 1)) / 6; res != exp {
			t.Fatalf("incorrect result %d, expected %d", res, exp)
		}
		log.Infof(context.Background(), "test query OK")
		return nil
	}
	if err := runQuery(); err != nil {
		t.Error(err)
	}

	// Verify the plan (should include all 5 nodes).
	r.CheckQueryResults(t,
		"SELECT info FROM [EXPLAIN (DISTSQL, SHAPE) SELECT sum(xsquared) FROM t] WHERE info LIKE 'Diagram%'",
		[][]string{{"Diagram: https://cockroachdb.github.io/distsqlplan/decode.html#eJyklW1ro0oYhr-fXyEPHNrCiI4aY_zU0nhoOOnLxiy7UEKZjU9dqXHSmZG2hPz3xaTdqlQxJoFAxpn7vrwGxw3I5xR8CINpcDnXkuyRa__Nbq-1--Dn3fRicqOdjifhPPw2JVp4dXEXnGnvU2W-On2VzzkTGJ3t16iF9uMqmAX7mOnk_0A7GScsFmz17wkQyHiEN2yFEvx7oEDAAgI2EHCAwAAWBNaCL1FKLoopm92CSfQKvkkgyda5KoYXBJZcIPgbUIlKEXyYs18pzpBFKAwTCESoWJIWk0Gdq4f1E74BgUue5qtM-toHNhAI16wY0Q3LhMWWAM_VZ41ULEbwaYlrMgbf3JLuaBdxLDBmigtjUCULv1-fntOzxlqrVjtorP1syzMuIhQYVaoW23Ywah5GZtfIaNUI7b5ZtM9mGZapG073_aKH0JW0uMftl1uptbpLsXpJcUzdcLtLsQ6hK0kZHidlWKm1u0uxe0lxTd3wukuxD6ErSfGOk-JVap3uUpxeUjxT72zEOQStZGR0nJFR41n3Re0M5ZpnEiuVTU1mrUmnxeGIUbx7K21A8lws8U7w5W7u_u_tztRuIEKp9ldpUSfVJPu4JJVAtvr7hign0dYkq5JEy0mDepLVznQIlN0a5TQn0XqS0_f23PrtDVqT3GYmq87k9mUa1pmGrUleM5NdZ_L6Mnl1plFrEjUrURUopw5Fzb5Uo-LReUz5y0MSgQ_m-0f_4ufjC8UCFsvi2Ah_85dd7PxtXTx9jyyVSOCaPeEYFYpVkiVSJUvwlchxu_3nzwCEq3Sv"}},
	)

	// Stop node 5.
	tc.StopServer(4)

	testutils.SucceedsSoon(t, runQuery)

	// The leaseholder for the last range should have moved to either node 2 or 3.
	query := "SELECT info FROM [EXPLAIN (DISTSQL, SHAPE) SELECT sum(xsquared) FROM t] WHERE info LIKE 'Diagram%'"
	exp2 := [][]string{{"Diagram: https://cockroachdb.github.io/distsqlplan/decode.html#eJyklG9ro0AQxt_fp1gGjrawQVeNyfmqpfFouPTPxRx3UELZi1MrNW66u9KWkO9-mLRXlSrGJBDI7vg8P5-Z3TWopwQ8CPyJfz4jcXovyPfp9SW59f_cTM7GV-R4NA5mwc8JJcHF2Y1_Qt5KVbY8flFPGZcYnuye0XPy-8Kf-juZyfiHT45GMY8kX349AgqpCPGKL1GBdwsMKFhAwQYKDswprKRYoFJC5tvrbfE4fAHPpBCnq0zny3MKCyERvDXoWCcIHsz43wSnyEOUhgkUQtQ8TvJi0Kf6bvWIr0DhXCTZMlUeeUcGCsGK5ys9wzJhvqEgMv1hozSPEDxW4BqPwDM3tD3aWRRJjLgW0nDKZMGvy-NTdlJra1VsnVrbD7csFTJEiWHJar5pBhvuB2ZXwIYlMNa-VaxLqwzL7BlO-26xfegKofQP61a_ZGu1D8XqFIpj9gzXJDwNCSNCP6BsHZC1D2khIPewgNySrd0-ILtTQK7ZM4btp8beh64QyuCwUAa1Z_wT2ymqlUgVlizrnMyKU4_llwKG0fYmXoMSmVzgjRSLbe3u7_V2fLYLISq922W5ndLj9H1LaYl8-f9mLCqxRiWrpMSKSk5VyWpU-rYHk92o5NQrsaqS0_Xt-tW36zcqufVMVpXJ7crkVpkGjUrDeia7yjTsyjTIZ_Q-Ec93cQgemG-f3ic_71_IH-CRys9n8CCet7Kz11U-5vc8UUjhkj_iCDXKZZzGSscL8LTMcLP58m8AqiLssA=="}}
	exp3 := [][]string{{"Diagram: https://cockroachdb.github.io/distsqlplan/decode.html#eJyklO9q2zAUxb_vKcSF0RYUbNmOk_lTS-PRsPTP4owNSihafOuaOlYqybQl5N2Hk3a1TW0cJ4FApKtzfjpX0hrUUwIeBP7EP5-ROL0X5Pv0-pLc-n9uJmfjK3I8Ggez4OeEkuDi7MY_IW-lKlsev6injEsMT3Zr9Jz8vvCn_k5mMv7hk6NRzCPJl1-PgEIqQrziS1Tg3QIDChZQsIGCA3MKKykWqJSQ-fR6WzwOX8AzKcTpKtP58JzCQkgEbw061gmCBzP-N8Ep8hClYQKFEDWPk7wY9Km-Wz3iK1A4F0m2TJVH3pGBQrDi-UjPsEyYbyiITH_YKM0jBI8VuMYj8MwNbY92FkUSI66FNJwyWfDr8viUndTaWhVbp9b2wy1LhQxRYliymm-awYb7gdkVsGEJjLVvFevSKsMye4ZjEp6GhBGhH1C27hzbh7QQUP-wzvVLtlb7gKxOATlmz3DbH2drH7pCKO5hobglW7t9KHanUFyzZwzbh2LvQ1cIZXBYKIPaO_6J7RTVSqQKS5Z1TmbFqcfyRwHDaPsSr0GJTC7wRorFtnb393p7p7YDISq9m2W5ndLj9H1KaYl8-f9lLCqxRiWrpMSKSk5VyWpU-rYHk92o5NQrsaqS03V3_eru-o1Kbj2TVWVyuzK5VaZBo9KwnsmuMg27Mg3yM3qfiOe7OAQPzLdP75Of9y_kC3ik8vsZPIjnrezsdZUf83ueKKRwyR9xhBrlMk5jpeMFeFpmuNl8-TcAsKjssA=="}}

	res := r.QueryStr(t, query)
	if !reflect.DeepEqual(res, exp2) && !reflect.DeepEqual(res, exp3) {
		t.Errorf("query '%s': expected:\neither\n%vor\n%v\ngot:\n%v\n",
			query, sqlutils.MatrixToStr(exp2), sqlutils.MatrixToStr(exp3), sqlutils.MatrixToStr(res),
		)
	}

	// Stop node 4; note that no range had replicas on both 4 and 5.
	tc.StopServer(3)

	testutils.SucceedsSoon(t, runQuery)
}

func TestDistSQLDrainingHosts(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	const numNodes = 2
	tc := serverutils.StartCluster(
		t,
		numNodes,
		base.TestClusterArgs{
			ReplicationMode: base.ReplicationManual,
			ServerArgs:      base.TestServerArgs{Knobs: base.TestingKnobs{DistSQL: &execinfra.TestingKnobs{DrainFast: true}}, UseDatabase: "test"},
		},
	)
	ctx := context.Background()
	defer tc.Stopper().Stop(ctx)

	conn := tc.ServerConn(0)
	sqlutils.CreateTable(
		t,
		conn,
		"nums",
		"num INT",
		numNodes, /* numRows */
		sqlutils.ToRowFn(sqlutils.RowIdxFn),
	)

	db := tc.ServerConn(0)
	db.SetMaxOpenConns(1)
	r := sqlutils.MakeSQLRunner(db)

	// Force the query to be distributed.
	r.Exec(t, "SET DISTSQL = ON")

	// Shortly after starting a cluster, the first server's StorePool may not be
	// fully initialized and ready to do rebalancing yet, so wrap this in a
	// SucceedsSoon.
	testutils.SucceedsSoon(t, func() error {
		_, err := db.Exec(
			fmt.Sprintf(`ALTER TABLE nums SPLIT AT VALUES (1);
									 ALTER TABLE nums EXPERIMENTAL_RELOCATE VALUES (ARRAY[%d], 1);`,
				tc.Server(1).GetFirstStoreID(),
			),
		)
		return err
	})

	// Ensure that the range cache is populated (see #31235).
	r.Exec(t, "SHOW RANGES FROM TABLE nums")

	const query = "SELECT count(*) FROM NUMS"
	expectPlan := func(expectedPlan [][]string) {
		t.Helper()
		planQuery := fmt.Sprintf(`SELECT info FROM [EXPLAIN (DISTSQL, SHAPE) %s] WHERE info LIKE 'Diagram%%'`, query)
		testutils.SucceedsSoon(t, func() error {
			resultPlan := r.QueryStr(t, planQuery)
			if !reflect.DeepEqual(resultPlan, expectedPlan) {
				return errors.Errorf("\nexpected:%v\ngot:%v", expectedPlan, resultPlan)
			}
			return nil
		})
	}

	// Verify distribution.
	expectPlan([][]string{{"Diagram: https://cockroachdb.github.io/distsqlplan/decode.html#eJysk99q2zAYxe_3FOaD0XQoJHKyG101JB41y7_FLh2UEDT7i2dqS54k05WQdx9y2jU2dUjGbBBYOj7nZx15B_pXBgwCb-qNQycVW-l8WS1mzoP3fTkd-XOnM_GDMPg2JU5wO1p6186LNJKlMJ1P1we5KHO9du5vvZV3MJn6Xz3napLyRPH84xUQEDLGOc9RA3sACgRcWBMolIxQa6ns9K4S-fFvYH0CqShKY6fXBCKpENgOTGoyBAYh_5HhCnmMqtcHAjEanmZWDBblxg6b4hGfgcBYZmUuNAMCQcGFZk63R2G9JyBL8xahDU8QGD1i8ifA-ntyPtYoSRQm3EjVc-tU48XdPNysFvdB57o1221ku63Zb5GlkCpGhXEtb70_TTes0wV3s40_Dzs3tB1u0IAb1uDo-X3Ri_vq0e7ZfdFLsI52ZPAf-hq09vVO9gp1IYXGWm5bUr-R1KW2YIyT6nfagZalinCpZFRpD4-LCrmaiFGbwyq1cdr44nVJG4U8_3vUj53oSSe35kSPndymk3vS6fMFTIOTTsN2J9p0Gv7r1w3s3m8z-bRJY2DQf7m67wyvN9gXeKLt4Qt-yqfKNnwubH1bnmkkMOOPOEGDKk9Fqk0aATOqxP3-w58BALvp0p8="}})

	// Drain the second node and expect the query to be planned on only the
	// first node.
	distServer := tc.Server(1).DistSQLServer().(*distsql.ServerImpl)
	distServer.Drain(ctx, 0 /* flowDrainWait */, nil /* reporter */)

	expectPlan([][]string{{"Diagram: https://cockroachdb.github.io/distsqlplan/decode.html#eJyUkVFr2zAUhd_3K8yB0WQo1O6jnhYSj5qlSWZ7dFBM0OwbT9SWPEmmK8H_fchpt7asYbXhgu89Pt_R1QH2ZwOOLF7FizyQaq-DT-nmKriJv21X82QdTJZJlmdfVizILufbeBo8SEvdKzf5MD3KVd_aIri-jNP4aLJKPsfB2VKK2oj2_RkYlK5oLVqy4DeIUDB0RpdkrTa-dRgFSfULPGSQquudbxcMpTYEfoCTriFw5OJ7QymJisx5CIaKnJCNF8PH-OjLrrulezAsdNO3ynIwZJ1QlgczFAOD7t1fgHWiJvDoSaJkCR4O7P9DzevaUC2cNufR80yLzdd1vks319lk-ir74gU7egs7JdtpZekZ9zVS-II0i4aCgap6vJgDrO5NSVujy1F7_NyMkcdGRdYdp5HHWZeox5F1hkT7Z3VPnaKTThennAqGfaPvdrICR_jwzP5RHl_4H0Rt_YqyH_putM3vO3_AvWgsMVyJW1qSI9NKJa2TJbgzPQ3Du98DAO68-qI="}})

	// Verify correctness.
	var res int
	if err := db.QueryRow(query).Scan(&res); err != nil {
		t.Fatal(err)
	}
	if res != numNodes {
		t.Fatalf("expected %d rows but got %d", numNodes, res)
	}
}

// testSpanResolverRange describes a range in a test. The ranges are specified
// in order, so only the start key is needed.
type testSpanResolverRange struct {
	startKey string
	node     int
}

// testSpanResolver is a SpanResolver that uses a fixed set of ranges.
type testSpanResolver struct {
	nodes []*roachpb.NodeDescriptor

	ranges []testSpanResolverRange
}

// NewSpanResolverIterator is part of the SpanResolver interface.
func (tsr *testSpanResolver) NewSpanResolverIterator(
	txn *kv.Txn, optionalOracle replicaoracle.Oracle,
) physicalplan.SpanResolverIterator {
	return &testSpanResolverIterator{tsr: tsr}
}

type testSpanResolverIterator struct {
	tsr         *testSpanResolver
	curRangeIdx int
	endKey      string
}

var _ physicalplan.SpanResolverIterator = &testSpanResolverIterator{}

// Seek is part of the SpanResolverIterator interface.
func (it *testSpanResolverIterator) Seek(
	ctx context.Context, span roachpb.Span, scanDir kvcoord.ScanDirection,
) {
	if scanDir != kvcoord.Ascending {
		panic("descending not implemented")
	}
	it.endKey = string(span.EndKey)
	key := string(span.Key)
	i := 0
	for ; i < len(it.tsr.ranges)-1; i++ {
		if key < it.tsr.ranges[i+1].startKey {
			break
		}
	}
	it.curRangeIdx = i
}

// Valid is part of the SpanResolverIterator interface.
func (*testSpanResolverIterator) Valid() bool {
	return true
}

// Error is part of the SpanResolverIterator interface.
func (*testSpanResolverIterator) Error() error {
	return nil
}

// NeedAnother is part of the SpanResolverIterator interface.
func (it *testSpanResolverIterator) NeedAnother() bool {
	return it.curRangeIdx < len(it.tsr.ranges)-1 &&
		it.tsr.ranges[it.curRangeIdx+1].startKey < it.endKey
}

// Next is part of the SpanResolverIterator interface.
func (it *testSpanResolverIterator) Next(_ context.Context) {
	if !it.NeedAnother() {
		panic("Next called with NeedAnother false")
	}
	it.curRangeIdx++
}

// Desc is part of the SpanResolverIterator interface.
func (it *testSpanResolverIterator) Desc() roachpb.RangeDescriptor {
	endKey := roachpb.RKeyMax
	if it.curRangeIdx < len(it.tsr.ranges)-1 {
		endKey = roachpb.RKey(it.tsr.ranges[it.curRangeIdx+1].startKey)
	}
	return roachpb.RangeDescriptor{
		StartKey: roachpb.RKey(it.tsr.ranges[it.curRangeIdx].startKey),
		EndKey:   endKey,
	}
}

// ReplicaInfo is part of the SpanResolverIterator interface.
func (it *testSpanResolverIterator) ReplicaInfo(
	_ context.Context,
) (roachpb.ReplicaDescriptor, bool, error) {
	n := it.tsr.nodes[it.tsr.ranges[it.curRangeIdx].node-1]
	return roachpb.ReplicaDescriptor{NodeID: n.NodeID}, false, nil
}

func TestPartitionSpans(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	testCases := []struct {
		ranges    []testSpanResolverRange
		deadNodes []int

		gatewayNode int

		// spans to be passed to PartitionSpans. If the second string is empty,
		// the span is actually a point lookup.
		spans [][2]string

		locFilter string

		// expected result: a map of node to list of spans.
		partitions      map[int][][2]string
		partitionStates []string
		partitionState  spanPartitionState
	}{
		{
			ranges:      []testSpanResolverRange{{"A", 1}, {"B", 2}, {"C", 1}, {"D", 3}},
			gatewayNode: 1,

			spans: [][2]string{{"A1", "C1"}, {"D1", "X"}},

			partitions: map[int][][2]string{
				1: {{"A1", "B"}, {"C", "C1"}},
				2: {{"B", "C"}},
				3: {{"D1", "X"}},
			},

			partitionStates: []string{
				"partition span: {A1-B}, instance ID: 1, reason: gossip-target-healthy",
				"partition span: {B-C}, instance ID: 2, reason: gossip-target-healthy",
				"partition span: C{-1}, instance ID: 1, reason: gossip-target-healthy",
				"partition span: {D1-X}, instance ID: 3, reason: gossip-target-healthy",
			},

			partitionState: spanPartitionState{
				partitionSpans: map[base.SQLInstanceID]int{
					1: 2,
					2: 1,
					3: 1,
				},
				partitionSpanDecisions: [SpanPartitionReason_LOCALITY_FILTERED_RANDOM_GATEWAY_OVERLOADED + 1]int{
					SpanPartitionReason_GOSSIP_TARGET_HEALTHY: 4,
				},
				totalPartitionSpans: 4,
			},
		},

		{
			ranges:      []testSpanResolverRange{{"A", 1}, {"B", 2}, {"C", 1}, {"D", 3}},
			deadNodes:   []int{1}, // The health status of the gateway node shouldn't matter.
			gatewayNode: 1,

			spans: [][2]string{{"A1", "C1"}, {"D1", "X"}},

			partitions: map[int][][2]string{
				1: {{"A1", "B"}, {"C", "C1"}},
				2: {{"B", "C"}},
				3: {{"D1", "X"}},
			},

			partitionStates: []string{
				"partition span: {A1-B}, instance ID: 1, reason: gossip-target-healthy",
				"partition span: {B-C}, instance ID: 2, reason: gossip-target-healthy",
				"partition span: C{-1}, instance ID: 1, reason: gossip-target-healthy",
				"partition span: {D1-X}, instance ID: 3, reason: gossip-target-healthy",
			},

			partitionState: spanPartitionState{
				partitionSpans: map[base.SQLInstanceID]int{
					1: 2,
					2: 1,
					3: 1,
				},
				partitionSpanDecisions: [SpanPartitionReason_LOCALITY_FILTERED_RANDOM_GATEWAY_OVERLOADED + 1]int{
					SpanPartitionReason_GOSSIP_TARGET_HEALTHY: 4,
				},
				totalPartitionSpans: 4,
			},
		},

		{
			ranges:      []testSpanResolverRange{{"A", 1}, {"B", 2}, {"C", 1}, {"D", 3}},
			deadNodes:   []int{2},
			gatewayNode: 1,

			spans: [][2]string{{"A1", "C1"}, {"D1", "X"}},

			partitions: map[int][][2]string{
				1: {{"A1", "C1"}},
				3: {{"D1", "X"}},
			},

			partitionStates: []string{
				"partition span: {A1-B}, instance ID: 1, reason: gossip-target-healthy",
				"partition span: {B-C}, instance ID: 1, reason: gossip-gateway-target-unhealthy",
				"partition span: C{-1}, instance ID: 1, reason: gossip-target-healthy",
				"partition span: {D1-X}, instance ID: 3, reason: gossip-target-healthy",
			},

			partitionState: spanPartitionState{
				partitionSpans: map[base.SQLInstanceID]int{
					1: 3,
					3: 1,
				},
				partitionSpanDecisions: [SpanPartitionReason_LOCALITY_FILTERED_RANDOM_GATEWAY_OVERLOADED + 1]int{
					SpanPartitionReason_GOSSIP_TARGET_HEALTHY:           3,
					SpanPartitionReason_GOSSIP_GATEWAY_TARGET_UNHEALTHY: 1,
				},
				totalPartitionSpans: 4,
			},
		},

		{
			ranges:      []testSpanResolverRange{{"A", 1}, {"B", 2}, {"C", 1}, {"D", 3}},
			deadNodes:   []int{3},
			gatewayNode: 1,

			spans: [][2]string{{"A1", "C1"}, {"D1", "X"}},

			partitions: map[int][][2]string{
				1: {{"A1", "B"}, {"C", "C1"}, {"D1", "X"}},
				2: {{"B", "C"}},
			},

			partitionStates: []string{
				"partition span: {A1-B}, instance ID: 1, reason: gossip-target-healthy",
				"partition span: {B-C}, instance ID: 2, reason: gossip-target-healthy",
				"partition span: C{-1}, instance ID: 1, reason: gossip-target-healthy",
				"partition span: {D1-X}, instance ID: 1, reason: gossip-gateway-target-unhealthy",
			},

			partitionState: spanPartitionState{
				partitionSpans: map[base.SQLInstanceID]int{
					1: 3,
					2: 1,
				},
				partitionSpanDecisions: [SpanPartitionReason_LOCALITY_FILTERED_RANDOM_GATEWAY_OVERLOADED + 1]int{
					SpanPartitionReason_GOSSIP_TARGET_HEALTHY:           3,
					SpanPartitionReason_GOSSIP_GATEWAY_TARGET_UNHEALTHY: 1,
				},
				totalPartitionSpans: 4,
			},
		},

		{
			ranges:      []testSpanResolverRange{{"A", 1}, {"B", 2}, {"C", 1}, {"D", 3}},
			deadNodes:   []int{1},
			gatewayNode: 2,

			spans: [][2]string{{"A1", "C1"}, {"D1", "X"}},

			partitions: map[int][][2]string{
				2: {{"A1", "C1"}},
				3: {{"D1", "X"}},
			},

			partitionStates: []string{
				"partition span: {A1-B}, instance ID: 2, reason: gossip-gateway-target-unhealthy",
				"partition span: {B-C}, instance ID: 2, reason: gossip-target-healthy",
				"partition span: C{-1}, instance ID: 2, reason: gossip-gateway-target-unhealthy",
				"partition span: {D1-X}, instance ID: 3, reason: gossip-target-healthy",
			},

			partitionState: spanPartitionState{
				partitionSpans: map[base.SQLInstanceID]int{
					2: 3,
					3: 1,
				},
				partitionSpanDecisions: [SpanPartitionReason_LOCALITY_FILTERED_RANDOM_GATEWAY_OVERLOADED + 1]int{
					SpanPartitionReason_GOSSIP_TARGET_HEALTHY:           2,
					SpanPartitionReason_GOSSIP_GATEWAY_TARGET_UNHEALTHY: 2,
				},
				totalPartitionSpans: 4,
			},
		},

		{
			ranges:      []testSpanResolverRange{{"A", 1}, {"B", 2}, {"C", 1}, {"D", 3}},
			deadNodes:   []int{1},
			gatewayNode: 3,

			spans: [][2]string{{"A1", "C1"}, {"D1", "X"}},

			partitions: map[int][][2]string{
				2: {{"B", "C"}},
				3: {{"A1", "B"}, {"C", "C1"}, {"D1", "X"}},
			},

			partitionStates: []string{
				"partition span: {A1-B}, instance ID: 3, reason: gossip-gateway-target-unhealthy",
				"partition span: {B-C}, instance ID: 2, reason: gossip-target-healthy",
				"partition span: C{-1}, instance ID: 3, reason: gossip-gateway-target-unhealthy",
				"partition span: {D1-X}, instance ID: 3, reason: gossip-target-healthy",
			},

			partitionState: spanPartitionState{
				partitionSpans: map[base.SQLInstanceID]int{
					2: 1,
					3: 3,
				},
				partitionSpanDecisions: [SpanPartitionReason_LOCALITY_FILTERED_RANDOM_GATEWAY_OVERLOADED + 1]int{
					SpanPartitionReason_GOSSIP_TARGET_HEALTHY:           2,
					SpanPartitionReason_GOSSIP_GATEWAY_TARGET_UNHEALTHY: 2,
				},
				totalPartitionSpans: 4,
			},
		},

		// Test point lookups in isolation.
		{
			ranges:      []testSpanResolverRange{{"A", 1}, {"B", 2}},
			gatewayNode: 1,

			spans: [][2]string{{"A2", ""}, {"A1", ""}, {"B1", ""}},

			partitions: map[int][][2]string{
				1: {{"A2", ""}, {"A1", ""}},
				2: {{"B1", ""}},
			},

			partitionStates: []string{
				"partition span: A2, instance ID: 1, reason: gossip-target-healthy",
				"partition span: A1, instance ID: 1, reason: gossip-target-healthy",
				"partition span: B1, instance ID: 2, reason: gossip-target-healthy",
			},

			partitionState: spanPartitionState{
				partitionSpans: map[base.SQLInstanceID]int{
					1: 2,
					2: 1,
				},
				partitionSpanDecisions: [SpanPartitionReason_LOCALITY_FILTERED_RANDOM_GATEWAY_OVERLOADED + 1]int{
					SpanPartitionReason_GOSSIP_TARGET_HEALTHY: 3,
				},
				totalPartitionSpans: 3,
			},
		},

		// Test point lookups intertwined with span scans.
		{
			ranges:      []testSpanResolverRange{{"A", 1}, {"B", 1}, {"C", 2}},
			gatewayNode: 1,

			spans: [][2]string{{"A1", ""}, {"A1", "A2"}, {"A2", ""}, {"A2", "C2"}, {"B1", ""}, {"A3", "B3"}, {"B2", ""}},

			partitions: map[int][][2]string{
				1: {{"A1", ""}, {"A1", "A2"}, {"A2", ""}, {"A2", "C"}, {"B1", ""}, {"A3", "B3"}, {"B2", ""}},
				2: {{"C", "C2"}},
			},

			partitionStates: []string{
				"partition span: A1, instance ID: 1, reason: gossip-target-healthy",
				"partition span: A{1-2}, instance ID: 1, reason: gossip-target-healthy",
				"partition span: A2, instance ID: 1, reason: gossip-target-healthy",
				"partition span: {A2-B}, instance ID: 1, reason: gossip-target-healthy",
				"partition span: {B-C}, instance ID: 1, reason: gossip-target-healthy",
				"partition span: C{-2}, instance ID: 2, reason: gossip-target-healthy",
				"partition span: B1, instance ID: 1, reason: gossip-target-healthy",
				"partition span: {A3-B}, instance ID: 1, reason: gossip-target-healthy",
				"partition span: B{-3}, instance ID: 1, reason: gossip-target-healthy",
				"partition span: B2, instance ID: 1, reason: gossip-target-healthy",
			},

			partitionState: spanPartitionState{
				partitionSpans: map[base.SQLInstanceID]int{
					1: 9,
					2: 1,
				},
				partitionSpanDecisions: [SpanPartitionReason_LOCALITY_FILTERED_RANDOM_GATEWAY_OVERLOADED + 1]int{
					SpanPartitionReason_GOSSIP_TARGET_HEALTHY: 10,
				},
				totalPartitionSpans: 10,
			},
		},

		// A single span touching multiple ranges but on the same node results
		// in a single partitioned span.
		{
			ranges:      []testSpanResolverRange{{"A", 1}, {"A1", 1}, {"B", 2}},
			gatewayNode: 1,

			spans: [][2]string{{"A", "B"}},

			partitions: map[int][][2]string{
				1: {{"A", "B"}},
			},

			partitionStates: []string{
				"partition span: A{-1}, instance ID: 1, reason: gossip-target-healthy",
				"partition span: {A1-B}, instance ID: 1, reason: gossip-target-healthy",
			},

			partitionState: spanPartitionState{
				partitionSpans: map[base.SQLInstanceID]int{
					1: 2,
				},
				partitionSpanDecisions: [SpanPartitionReason_LOCALITY_FILTERED_RANDOM_GATEWAY_OVERLOADED + 1]int{
					SpanPartitionReason_GOSSIP_TARGET_HEALTHY: 2,
				},
				totalPartitionSpans: 2,
			},
		},
		// Test some locality-filtered planning too.
		//
		// Since this test is run on a system tenant but there is a locality filter,
		// the spans are resolved in a mixed process mode. As a result, the
		// partition states include a mix of locality and target reasons, which is
		// expected.
		{
			ranges:      []testSpanResolverRange{{"A", 1}, {"B", 2}, {"C", 1}, {"D", 3}},
			gatewayNode: 1,

			spans:     [][2]string{{"A1", "C1"}, {"D1", "X"}},
			locFilter: "x=1",
			partitions: map[int][][2]string{
				1: {{"A1", "B"}, {"C", "C1"}},
				2: {{"B", "C"}, {"D1", "X"}},
			},

			partitionStates: []string{
				"partition span: {A1-B}, instance ID: 1, reason: target-healthy",
				"partition span: {B-C}, instance ID: 2, reason: target-healthy",
				"partition span: C{-1}, instance ID: 1, reason: target-healthy",
				"partition span: {D1-X}, instance ID: 2, reason: locality-filtered-random-gateway-overloaded",
			},

			partitionState: spanPartitionState{
				partitionSpans: map[base.SQLInstanceID]int{
					1: 2,
					2: 2,
				},
				partitionSpanDecisions: [SpanPartitionReason_LOCALITY_FILTERED_RANDOM_GATEWAY_OVERLOADED + 1]int{
					SpanPartitionReason_TARGET_HEALTHY:                              3,
					SpanPartitionReason_LOCALITY_FILTERED_RANDOM_GATEWAY_OVERLOADED: 1,
				},
				testingOverrideRandomSelection: func() base.SQLInstanceID {
					return 2
				},
				totalPartitionSpans: 4,
			},
		},
		{
			ranges:      []testSpanResolverRange{{"A", 1}, {"B", 2}, {"C", 1}, {"D", 3}},
			gatewayNode: 1,

			spans:     [][2]string{{"A1", "C1"}, {"D1", "X"}},
			locFilter: "y=0",
			partitions: map[int][][2]string{
				2: {{"A1", "C1"}},
				4: {{"D1", "X"}},
			},

			partitionStates: []string{
				"partition span: {A1-B}, instance ID: 2, reason: closest-locality-match",
				"partition span: {B-C}, instance ID: 2, reason: target-healthy",
				"partition span: C{-1}, instance ID: 2, reason: closest-locality-match",
				"partition span: {D1-X}, instance ID: 4, reason: closest-locality-match",
			},

			partitionState: spanPartitionState{
				partitionSpans: map[base.SQLInstanceID]int{
					2: 3,
					4: 1,
				},
				partitionSpanDecisions: [SpanPartitionReason_LOCALITY_FILTERED_RANDOM_GATEWAY_OVERLOADED + 1]int{
					SpanPartitionReason_TARGET_HEALTHY:         1,
					SpanPartitionReason_CLOSEST_LOCALITY_MATCH: 3,
				},
				totalPartitionSpans: 4,
			},
		},
		{
			ranges:      []testSpanResolverRange{{"A", 1}, {"B", 2}, {"C", 1}, {"D", 3}},
			gatewayNode: 7,

			spans:     [][2]string{{"A1", "C1"}, {"D1", "X"}},
			locFilter: "x=3",
			partitions: map[int][][2]string{
				6: {{"B", "C1"}},
				7: {{"A1", "B"}, {"D1", "X"}},
			},

			partitionStates: []string{
				"partition span: {A1-B}, instance ID: 7, reason: gateway-no-locality-match",
				"partition span: {B-C}, instance ID: 6, reason: locality-filtered-random-gateway-overloaded",
				"partition span: C{-1}, instance ID: 6, reason: locality-filtered-random-gateway-overloaded",
				"partition span: {D1-X}, instance ID: 7, reason: gateway-no-locality-match",
			},

			partitionState: spanPartitionState{
				partitionSpans: map[base.SQLInstanceID]int{
					6: 2,
					7: 2,
				},
				partitionSpanDecisions: [SpanPartitionReason_LOCALITY_FILTERED_RANDOM_GATEWAY_OVERLOADED + 1]int{
					SpanPartitionReason_GATEWAY_NO_LOCALITY_MATCH:                   2,
					SpanPartitionReason_LOCALITY_FILTERED_RANDOM_GATEWAY_OVERLOADED: 2,
				},
				totalPartitionSpans: 4,
				testingOverrideRandomSelection: func() base.SQLInstanceID {
					return 6
				},
			},
		},
		{
			ranges:      []testSpanResolverRange{{"A", 1}, {"B", 2}, {"C", 1}, {"D", 3}},
			gatewayNode: 1,

			spans:     [][2]string{{"A1", "C1"}, {"D1", "X"}},
			locFilter: "x=3,y=1",
			partitions: map[int][][2]string{
				7: {{"A1", "C1"}, {"D1", "X"}},
			},

			partitionStates: []string{
				"partition span: {A1-B}, instance ID: 7, reason: locality-filtered-random",
				"partition span: {B-C}, instance ID: 7, reason: locality-filtered-random",
				"partition span: C{-1}, instance ID: 7, reason: locality-filtered-random",
				"partition span: {D1-X}, instance ID: 7, reason: locality-filtered-random",
			},

			partitionState: spanPartitionState{
				partitionSpans: map[base.SQLInstanceID]int{
					7: 4,
				},
				partitionSpanDecisions: [SpanPartitionReason_LOCALITY_FILTERED_RANDOM_GATEWAY_OVERLOADED + 1]int{
					SpanPartitionReason_LOCALITY_FILTERED_RANDOM: 4,
				},
				totalPartitionSpans: 4,
			},
		},
	}

	// We need a mock Gossip to contain addresses for the nodes. Otherwise the
	// DistSQLPlanner will not plan flows on them.
	ctx := context.Background()
	s := serverutils.StartServerOnly(t, base.TestServerArgs{
		Knobs: base.TestingKnobs{
			DistSQL: &execinfra.TestingKnobs{
				MinimumNumberOfGatewayPartitions: 1,
			},
		},
	})
	defer s.Stopper().Stop(ctx)
	mockGossip := gossip.NewTest(roachpb.NodeID(1), s.Stopper(), metric.NewRegistry())
	var nodeDescs []*roachpb.NodeDescriptor
	mockInstances := make(mockAddressResolver)
	for i := 1; i <= 10; i++ {
		sqlInstanceID := base.SQLInstanceID(i)
		var l roachpb.Locality
		require.NoError(t, l.Set(fmt.Sprintf("x=%d,y=%d", (i/3)+1, i%2)))
		desc := &roachpb.NodeDescriptor{
			NodeID:   roachpb.NodeID(sqlInstanceID),
			Address:  util.UnresolvedAddr{AddressField: fmt.Sprintf("addr%d", i)},
			Locality: l,
		}
		mockInstances[sqlInstanceID] = l
		if err := mockGossip.SetNodeDescriptor(desc); err != nil {
			t.Fatal(err)
		}
		if err := mockGossip.AddInfoProto(
			gossip.MakeDistSQLNodeVersionKey(sqlInstanceID),
			&execinfrapb.DistSQLVersionGossipInfo{
				MinAcceptedVersion: execinfra.MinAcceptedVersion,
				Version:            execinfra.Version,
			},
			0, // ttl - no expiration
		); err != nil {
			t.Fatal(err)
		}

		nodeDescs = append(nodeDescs, desc)
	}

	for testIdx, tc := range testCases {
		t.Run(strconv.Itoa(testIdx), func(t *testing.T) {
			ctx := context.Background()
			ctx, getRecAndFinish := tracing.ContextWithRecordingSpan(ctx, s.Tracer(), "PartitionSpans")

			stopper := stop.NewStopper()
			defer stopper.Stop(context.Background())

			tsp := &testSpanResolver{
				nodes:  nodeDescs,
				ranges: tc.ranges,
			}

			nID := &base.NodeIDContainer{}
			nID.Reset(tsp.nodes[tc.gatewayNode-1].NodeID)

			gw := gossip.MakeOptionalGossip(mockGossip)
			dsp := DistSQLPlanner{
				planVersion:          execinfra.Version,
				st:                   cluster.MakeTestingClusterSettings(),
				gatewaySQLInstanceID: base.SQLInstanceID(tsp.nodes[tc.gatewayNode-1].NodeID),
				stopper:              stopper,
				spanResolver:         tsp,
				gossip:               gw,
				nodeHealth: distSQLNodeHealth{
					gossip: gw,
					connHealth: func(node roachpb.NodeID, _ rpc.ConnectionClass) error {
						for _, n := range tc.deadNodes {
							if int(node) == n {
								return fmt.Errorf("test node is unhealthy")
							}
						}
						return nil
					},
					isAvailable: func(base.SQLInstanceID) bool {
						return true
					},
				},
				sqlAddressResolver: mockInstances,
				distSQLSrv: &distsql.ServerImpl{
					ServerConfig: execinfra.ServerConfig{
						NodeID:       base.NewSQLIDContainerForNode(nID),
						TestingKnobs: execinfra.TestingKnobs{MinimumNumberOfGatewayPartitions: 1},
					},
				},
				codec:     keys.SystemSQLCodec,
				nodeDescs: mockGossip,
			}

			var locFilter roachpb.Locality
			if tc.locFilter != "" {
				require.NoError(t, locFilter.Set(tc.locFilter))
			}
			evalCtx := &eval.Context{
				Codec: keys.SystemSQLCodec,
				SessionDataStack: sessiondata.NewStack(&sessiondata.SessionData{
					SessionData: sessiondatapb.SessionData{
						DistsqlPlanGatewayBias: 2,
					},
				}),
			}
			planCtx := dsp.NewPlanningCtxWithOracle(ctx, &extendedEvalContext{
				Context: *evalCtx,
			}, nil, nil, DistributionTypeSystemTenantOnly, physicalplan.DefaultReplicaChooser, locFilter)
			planCtx.spanPartitionState.testingOverrideRandomSelection = tc.partitionState.testingOverrideRandomSelection
			var spans []roachpb.Span
			for _, s := range tc.spans {
				spans = append(spans, roachpb.Span{Key: roachpb.Key(s[0]), EndKey: roachpb.Key(s[1])})
			}

			partitions, err := dsp.PartitionSpans(ctx, planCtx, spans)
			if err != nil {
				t.Fatal(err)
			}

			// Assert that the PartitionState is what we expect it to be.
			tc.partitionState.testingOverrideRandomSelection = nil
			planCtx.spanPartitionState.testingOverrideRandomSelection = nil
			if !reflect.DeepEqual(*planCtx.spanPartitionState, tc.partitionState) {
				t.Errorf("expected partition state:\n  %v\ngot:\n  %v",
					tc.partitionState, *planCtx.spanPartitionState)
			}

			resMap := make(map[int][][2]string)
			for _, p := range partitions {
				if _, ok := resMap[int(p.SQLInstanceID)]; ok {
					t.Fatalf("node %d shows up in multiple partitions", p)
				}
				var spans [][2]string
				for _, s := range p.Spans {
					spans = append(spans, [2]string{string(s.Key), string(s.EndKey)})
				}
				resMap[int(p.SQLInstanceID)] = spans
			}

			recording := getRecAndFinish()
			t.Logf("recording is %s", recording)
			for _, expectedMsg := range tc.partitionStates {
				require.NotEqual(t, -1, tracing.FindMsgInRecording(recording, expectedMsg))
			}

			if !reflect.DeepEqual(resMap, tc.partitions) {
				t.Errorf("expected partitions:\n  %v\ngot:\n  %v", tc.partitions, resMap)
			}
		})
	}
}

// TestShouldPickGatewayNode is a unit test of the shouldPickGateway method.
func TestShouldPickGatewayNode(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	testCases := []struct {
		name            string
		gatewayInstance base.SQLInstanceID
		instances       []sqlinstance.InstanceInfo
		partitionState  *spanPartitionState
		shouldPick      bool
	}{
		{
			name:            "no_instances",
			gatewayInstance: 1,
			instances:       []sqlinstance.InstanceInfo{},
			partitionState: &spanPartitionState{partitionSpans: map[base.SQLInstanceID]int{
				1: 5,
			}},
			shouldPick: true,
		},
		{
			name:            "only_gateway",
			gatewayInstance: 1,
			instances: []sqlinstance.InstanceInfo{
				{
					InstanceID: base.SQLInstanceID(1),
				},
			},
			partitionState: &spanPartitionState{partitionSpans: map[base.SQLInstanceID]int{
				1: 5,
			}},
			shouldPick: true,
		},
		{
			name:            "gateway_0",
			gatewayInstance: 1,
			instances: []sqlinstance.InstanceInfo{
				{
					InstanceID: base.SQLInstanceID(1),
				},
				{
					InstanceID: base.SQLInstanceID(2),
				},
				{
					InstanceID: base.SQLInstanceID(3),
				},
			},
			partitionState: &spanPartitionState{partitionSpans: map[base.SQLInstanceID]int{
				1: 0,
				2: 0,
				3: 0,
			}},
			shouldPick: true,
		},
		{
			name:            "gateway_0_others_non_zero",
			gatewayInstance: 1,
			instances: []sqlinstance.InstanceInfo{
				{
					InstanceID: base.SQLInstanceID(1),
				},
				{
					InstanceID: base.SQLInstanceID(2),
				},
				{
					InstanceID: base.SQLInstanceID(3),
				},
			},
			partitionState: &spanPartitionState{partitionSpans: map[base.SQLInstanceID]int{
				1: 0,
				2: 1,
				3: 1,
			}},
			shouldPick: true,
		},
		{
			name:            "below_threshold_1",
			gatewayInstance: 1,
			instances: []sqlinstance.InstanceInfo{
				{
					InstanceID: base.SQLInstanceID(1),
				},
				{
					InstanceID: base.SQLInstanceID(2),
				},
				{
					InstanceID: base.SQLInstanceID(3),
				},
			},
			partitionState: &spanPartitionState{partitionSpans: map[base.SQLInstanceID]int{
				1: 1,
				2: 1,
				3: 1,
			}},
			shouldPick: true,
		},
		{
			name:            "above_threshold_1",
			gatewayInstance: 1,
			instances: []sqlinstance.InstanceInfo{
				{
					InstanceID: base.SQLInstanceID(1),
				},
				{
					InstanceID: base.SQLInstanceID(2),
				},
				{
					InstanceID: base.SQLInstanceID(3),
				},
			},
			partitionState: &spanPartitionState{partitionSpans: map[base.SQLInstanceID]int{
				1: 1,
				2: 1,
				3: 0,
			}},
			shouldPick: false,
		},
		{
			name:            "above_threshold_2",
			gatewayInstance: 1,
			instances: []sqlinstance.InstanceInfo{
				{
					InstanceID: base.SQLInstanceID(1),
				},
				{
					InstanceID: base.SQLInstanceID(2),
				},
				{
					InstanceID: base.SQLInstanceID(3),
				},
			},
			partitionState: &spanPartitionState{partitionSpans: map[base.SQLInstanceID]int{
				1: 2,
				2: 1,
				3: 1,
			}},
			shouldPick: false,
		},
		{
			name:            "above_threshold_3",
			gatewayInstance: 1,
			instances: []sqlinstance.InstanceInfo{
				{
					InstanceID: base.SQLInstanceID(1),
				},
				{
					InstanceID: base.SQLInstanceID(2),
				},
				{
					InstanceID: base.SQLInstanceID(3),
				},
				{
					InstanceID: base.SQLInstanceID(4),
				},
			},
			partitionState: &spanPartitionState{partitionSpans: map[base.SQLInstanceID]int{
				1: 4,
				2: 1,
				3: 0,
				4: 5,
			}},
			shouldPick: false,
		},
	}

	for _, tc := range testCases {
		mockDsp := &DistSQLPlanner{
			gatewaySQLInstanceID: tc.gatewayInstance,
			distSQLSrv: &distsql.ServerImpl{
				ServerConfig: execinfra.ServerConfig{
					TestingKnobs: execinfra.TestingKnobs{MinimumNumberOfGatewayPartitions: 1},
				},
			},
		}
		evalCtx := &eval.Context{
			SessionDataStack: sessiondata.NewStack(&sessiondata.SessionData{
				SessionData: sessiondatapb.SessionData{
					DistsqlPlanGatewayBias: 2,
				},
			}),
		}
		mockPlanCtx := &PlanningCtx{
			ExtendedEvalCtx: &extendedEvalContext{
				Context: *evalCtx,
			},
		}
		t.Run(tc.name, func(t *testing.T) {
			mockPlanCtx.spanPartitionState = tc.partitionState
			for _, partitionCount := range tc.partitionState.partitionSpans {
				mockPlanCtx.spanPartitionState.totalPartitionSpans += partitionCount
			}
			shouldPick := mockDsp.shouldPickGateway(mockPlanCtx, tc.instances)
			require.Equal(t, tc.shouldPick, shouldPick)
		})
	}
}

// Test that span partitioning takes into account the advertised acceptable
// versions of each node. Spans for which the owner node doesn't support our
// plan's version will be planned on the gateway.
func TestPartitionSpansSkipsIncompatibleNodes(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	// The spans that we're going to plan for.
	span := roachpb.Span{Key: roachpb.Key("A"), EndKey: roachpb.Key("Z")}
	gatewayNode := roachpb.NodeID(2)
	ranges := []testSpanResolverRange{{"A", 1}, {"B", 2}, {"C", 1}}

	testCases := []struct {
		// the test's name
		name string

		// planVersion is the DistSQL version that this plan is targeting.
		// We'll play with this version and expect nodes to be skipped because of
		// this.
		planVersion execinfrapb.DistSQLVersion

		// The versions accepted by each node.
		nodeVersions map[base.SQLInstanceID]execinfrapb.DistSQLVersionGossipInfo

		// nodesNotAdvertisingDistSQLVersion is the set of nodes for which gossip is
		// not going to have information about the supported DistSQL version. This
		// is to simulate CRDB 1.0 nodes which don't advertise this information.
		nodesNotAdvertisingDistSQLVersion map[base.SQLInstanceID]struct{}

		// expected result: a map of node to list of spans.
		partitions map[base.SQLInstanceID][][2]string
	}{
		{
			// In the first test, all nodes are compatible.
			name:        "current_version",
			planVersion: 2,
			nodeVersions: map[base.SQLInstanceID]execinfrapb.DistSQLVersionGossipInfo{
				1: {
					MinAcceptedVersion: 1,
					Version:            2,
				},
				2: {
					MinAcceptedVersion: 1,
					Version:            2,
				},
			},
			partitions: map[base.SQLInstanceID][][2]string{
				1: {{"A", "B"}, {"C", "Z"}},
				2: {{"B", "C"}},
			},
		},
		{
			// Plan version is incompatible with node 1. We expect everything to be
			// assigned to the gateway.
			// Remember that the gateway is node 2.
			name:        "next_version",
			planVersion: 3,
			nodeVersions: map[base.SQLInstanceID]execinfrapb.DistSQLVersionGossipInfo{
				1: {
					MinAcceptedVersion: 1,
					Version:            2,
				},
				2: {
					MinAcceptedVersion: 3,
					Version:            3,
				},
			},
			partitions: map[base.SQLInstanceID][][2]string{
				2: {{"A", "Z"}},
			},
		},
		{
			// Like the above, except node 1 is not gossiping its version (simulating
			// a crdb 1.0 node).
			name:        "crdb_1.0",
			planVersion: 3,
			nodeVersions: map[base.SQLInstanceID]execinfrapb.DistSQLVersionGossipInfo{
				2: {
					MinAcceptedVersion: 3,
					Version:            3,
				},
			},
			nodesNotAdvertisingDistSQLVersion: map[base.SQLInstanceID]struct{}{
				1: {},
			},
			partitions: map[base.SQLInstanceID][][2]string{
				2: {{"A", "Z"}},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			stopper := stop.NewStopper()
			defer stopper.Stop(context.Background())

			// We need a mock Gossip to contain addresses for the nodes. Otherwise the
			// DistSQLPlanner will not plan flows on them. This Gossip will also
			// reflect tc.nodesNotAdvertisingDistSQLVersion.
			testStopper := stop.NewStopper()
			defer testStopper.Stop(context.Background())
			mockGossip := gossip.NewTest(roachpb.NodeID(1), testStopper, metric.NewRegistry())
			var nodeDescs []*roachpb.NodeDescriptor
			for i := 1; i <= 2; i++ {
				sqlInstanceID := base.SQLInstanceID(i)
				desc := &roachpb.NodeDescriptor{
					NodeID:  roachpb.NodeID(sqlInstanceID),
					Address: util.UnresolvedAddr{AddressField: fmt.Sprintf("addr%d", i)},
				}
				if err := mockGossip.SetNodeDescriptor(desc); err != nil {
					t.Fatal(err)
				}
				if _, ok := tc.nodesNotAdvertisingDistSQLVersion[sqlInstanceID]; !ok {
					verInfo := tc.nodeVersions[sqlInstanceID]
					if err := mockGossip.AddInfoProto(
						gossip.MakeDistSQLNodeVersionKey(sqlInstanceID),
						&verInfo,
						0, // ttl - no expiration
					); err != nil {
						t.Fatal(err)
					}
				}

				nodeDescs = append(nodeDescs, desc)
			}
			tsp := &testSpanResolver{
				nodes:  nodeDescs,
				ranges: ranges,
			}

			gw := gossip.MakeOptionalGossip(mockGossip)
			dsp := DistSQLPlanner{
				planVersion:          tc.planVersion,
				st:                   cluster.MakeTestingClusterSettings(),
				gatewaySQLInstanceID: base.SQLInstanceID(tsp.nodes[gatewayNode-1].NodeID),
				stopper:              stopper,
				spanResolver:         tsp,
				gossip:               gw,
				nodeHealth: distSQLNodeHealth{
					gossip: gw,
					connHealth: func(roachpb.NodeID, rpc.ConnectionClass) error {
						// All the nodes are healthy.
						return nil
					},
					isAvailable: func(base.SQLInstanceID) bool {
						return true
					},
				},
				codec: keys.SystemSQLCodec,
			}

			ctx := context.Background()
			planCtx := dsp.NewPlanningCtx(ctx, &extendedEvalContext{
				Context: eval.Context{Codec: keys.SystemSQLCodec},
			}, nil, nil, DistributionTypeSystemTenantOnly)
			partitions, err := dsp.PartitionSpans(ctx, planCtx, roachpb.Spans{span})
			if err != nil {
				t.Fatal(err)
			}

			resMap := make(map[base.SQLInstanceID][][2]string)
			for _, p := range partitions {
				if _, ok := resMap[p.SQLInstanceID]; ok {
					t.Fatalf("node %d shows up in multiple partitions", p)
				}
				var spans [][2]string
				for _, s := range p.Spans {
					spans = append(spans, [2]string{string(s.Key), string(s.EndKey)})
				}
				resMap[p.SQLInstanceID] = spans
			}

			if !reflect.DeepEqual(resMap, tc.partitions) {
				t.Errorf("expected partitions:\n  %v\ngot:\n  %v", tc.partitions, resMap)
			}
		})
	}
}

// Test that a node whose descriptor info is not accessible through gossip is
// not used. This is to simulate nodes that have been decomisioned and also
// nodes that have been "replaced" by another node at the same address (which, I
// guess, is also a type of decomissioning).
func TestPartitionSpansSkipsNodesNotInGossip(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	// The spans that we're going to plan for.
	span := roachpb.Span{Key: roachpb.Key("A"), EndKey: roachpb.Key("Z")}
	gatewayNode := roachpb.NodeID(2)
	ranges := []testSpanResolverRange{{"A", 1}, {"B", 2}, {"C", 1}}

	stopper := stop.NewStopper()
	defer stopper.Stop(context.Background())

	mockGossip := gossip.NewTest(roachpb.NodeID(1), stopper, metric.NewRegistry())
	var nodeDescs []*roachpb.NodeDescriptor
	for i := 1; i <= 2; i++ {
		sqlInstanceID := base.SQLInstanceID(i)
		desc := &roachpb.NodeDescriptor{
			NodeID:  roachpb.NodeID(sqlInstanceID),
			Address: util.UnresolvedAddr{AddressField: fmt.Sprintf("addr%d", i)},
		}
		if i == 2 {
			if err := mockGossip.SetNodeDescriptor(desc); err != nil {
				t.Fatal(err)
			}
		}
		// All the nodes advertise their DistSQL versions. This is to simulate the
		// "node overridden by another node at the same address" case mentioned in
		// the test comment - for such a node, the descriptor would be taken out of
		// the gossip data, but other datums it advertised are left in place.
		if err := mockGossip.AddInfoProto(
			gossip.MakeDistSQLNodeVersionKey(sqlInstanceID),
			&execinfrapb.DistSQLVersionGossipInfo{
				MinAcceptedVersion: execinfra.MinAcceptedVersion,
				Version:            execinfra.Version,
			},
			0, // ttl - no expiration
		); err != nil {
			t.Fatal(err)
		}

		nodeDescs = append(nodeDescs, desc)
	}
	tsp := &testSpanResolver{
		nodes:  nodeDescs,
		ranges: ranges,
	}

	gw := gossip.MakeOptionalGossip(mockGossip)
	dsp := DistSQLPlanner{
		planVersion:          execinfra.Version,
		st:                   cluster.MakeTestingClusterSettings(),
		gatewaySQLInstanceID: base.SQLInstanceID(tsp.nodes[gatewayNode-1].NodeID),
		stopper:              stopper,
		spanResolver:         tsp,
		gossip:               gw,
		nodeHealth: distSQLNodeHealth{
			gossip: gw,
			connHealth: func(node roachpb.NodeID, _ rpc.ConnectionClass) error {
				_, _, err := mockGossip.GetNodeIDAddress(node)
				return err
			},
			isAvailable: func(base.SQLInstanceID) bool {
				return true
			},
		},
		codec: keys.SystemSQLCodec,
	}

	ctx := context.Background()
	planCtx := dsp.NewPlanningCtx(ctx, &extendedEvalContext{
		Context: eval.Context{Codec: keys.SystemSQLCodec},
	}, nil, nil, DistributionTypeSystemTenantOnly)
	partitions, err := dsp.PartitionSpans(ctx, planCtx, roachpb.Spans{span})
	if err != nil {
		t.Fatal(err)
	}

	resMap := make(map[base.SQLInstanceID][][2]string)
	for _, p := range partitions {
		if _, ok := resMap[p.SQLInstanceID]; ok {
			t.Fatalf("node %d shows up in multiple partitions", p)
		}
		var spans [][2]string
		for _, s := range p.Spans {
			spans = append(spans, [2]string{string(s.Key), string(s.EndKey)})
		}
		resMap[p.SQLInstanceID] = spans
	}

	expectedPartitions :=
		map[base.SQLInstanceID][][2]string{
			2: {{"A", "Z"}},
		}
	if !reflect.DeepEqual(resMap, expectedPartitions) {
		t.Errorf("expected partitions:\n  %v\ngot:\n  %v", expectedPartitions, resMap)
	}
}

func TestCheckNodeHealth(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	stopper := stop.NewStopper()
	defer stopper.Stop(context.Background())

	const sqlInstanceID = base.SQLInstanceID(5)

	mockGossip := gossip.NewTest(roachpb.NodeID(sqlInstanceID), stopper, metric.NewRegistry())

	desc := &roachpb.NodeDescriptor{
		NodeID:  roachpb.NodeID(sqlInstanceID),
		Address: util.UnresolvedAddr{NetworkField: "tcp", AddressField: "testaddr"},
	}
	if err := mockGossip.SetNodeDescriptor(desc); err != nil {
		t.Fatal(err)
	}
	if err := mockGossip.AddInfoProto(
		gossip.MakeDistSQLNodeVersionKey(sqlInstanceID),
		&execinfrapb.DistSQLVersionGossipInfo{
			MinAcceptedVersion: execinfra.MinAcceptedVersion,
			Version:            execinfra.Version,
		},
		0, // ttl - no expiration
	); err != nil {
		t.Fatal(err)
	}

	notAvailable := func(base.SQLInstanceID) bool {
		return false
	}
	available := func(base.SQLInstanceID) bool {
		return true
	}

	connHealthy := func(roachpb.NodeID, rpc.ConnectionClass) error {
		return nil
	}
	connUnhealthy := func(roachpb.NodeID, rpc.ConnectionClass) error {
		return errors.New("injected conn health error")
	}
	_ = connUnhealthy

	livenessTests := []struct {
		isAvailable func(id base.SQLInstanceID) bool
		exp         string
	}{
		{available, ""},
		{notAvailable, "not using n5 since it is not available"},
	}

	gw := gossip.MakeOptionalGossip(mockGossip)
	for _, test := range livenessTests {
		t.Run("liveness", func(t *testing.T) {
			h := distSQLNodeHealth{
				gossip:      gw,
				connHealth:  connHealthy,
				isAvailable: test.isAvailable,
			}
			if err := h.checkSystem(context.Background(), sqlInstanceID); !testutils.IsError(err, test.exp) {
				t.Fatalf("expected %v, got %v", test.exp, err)
			}
		})
	}

	connHealthTests := []struct {
		connHealth func(roachpb.NodeID, rpc.ConnectionClass) error
		exp        string
	}{
		{connHealthy, ""},
		{connUnhealthy, "injected conn health error"},
	}

	for _, test := range connHealthTests {
		t.Run("connHealth", func(t *testing.T) {
			h := distSQLNodeHealth{
				gossip:      gw,
				connHealth:  test.connHealth,
				isAvailable: available,
			}
			if err := h.checkSystem(context.Background(), sqlInstanceID); !testutils.IsError(err, test.exp) {
				t.Fatalf("expected %v, got %v", test.exp, err)
			}
		})
	}
}

func TestCheckScanParallelizationIfLocal(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	makeTableDesc := func() catalog.TableDescriptor {
		tableDesc := descpb.TableDescriptor{
			PrimaryIndex: descpb.IndexDescriptor{},
		}
		b := tabledesc.NewBuilder(&tableDesc)
		require.NoError(t, b.RunPostDeserializationChanges())
		return b.BuildImmutableTable()
	}

	scanToParallelize := &scanNode{parallelize: true}
	for _, tc := range []struct {
		plan                     planComponents
		prohibitParallelization  bool
		hasScanNodeToParallelize bool
	}{
		{
			plan: planComponents{main: planMaybePhysical{planNode: &scanNode{}}},
			// scanNode.parallelize is not set.
			hasScanNodeToParallelize: false,
		},
		{
			plan: planComponents{main: planMaybePhysical{planNode: &scanNode{parallelize: true, reqOrdering: ReqOrdering{{}}}}},
			// scanNode.reqOrdering is not empty.
			hasScanNodeToParallelize: false,
		},
		{
			plan:                     planComponents{main: planMaybePhysical{planNode: scanToParallelize}},
			hasScanNodeToParallelize: true,
		},
		{
			plan:                     planComponents{main: planMaybePhysical{planNode: &distinctNode{plan: scanToParallelize}}},
			hasScanNodeToParallelize: true,
		},
		{
			plan: planComponents{main: planMaybePhysical{planNode: &filterNode{source: planDataSource{plan: scanToParallelize}}}},
			// filterNode might be handled via wrapping a row-execution
			// processor, so we safely prohibit the parallelization.
			prohibitParallelization: true,
		},
		{
			plan: planComponents{main: planMaybePhysical{planNode: &groupNode{
				plan:  scanToParallelize,
				funcs: []*aggregateFuncHolder{{filterRenderIdx: tree.NoColumnIdx}}},
			}},
			// Non-filtering aggregation is supported.
			hasScanNodeToParallelize: true,
		},
		{
			plan: planComponents{main: planMaybePhysical{planNode: &groupNode{
				plan:  scanToParallelize,
				funcs: []*aggregateFuncHolder{{filterRenderIdx: 0}}},
			}},
			// Filtering aggregation is not natively supported.
			prohibitParallelization: true,
		},
		{
			plan: planComponents{main: planMaybePhysical{planNode: &indexJoinNode{
				input: scanToParallelize,
				table: &scanNode{desc: makeTableDesc()},
			}}},
			hasScanNodeToParallelize: true,
		},
		{
			plan:                     planComponents{main: planMaybePhysical{planNode: &limitNode{plan: scanToParallelize}}},
			hasScanNodeToParallelize: true,
		},
		{
			plan:                     planComponents{main: planMaybePhysical{planNode: &ordinalityNode{source: scanToParallelize}}},
			hasScanNodeToParallelize: true,
		},
		{
			plan: planComponents{main: planMaybePhysical{planNode: &renderNode{
				source: planDataSource{plan: scanToParallelize},
				render: []tree.TypedExpr{&tree.IndexedVar{Idx: 0}},
			}}},
			hasScanNodeToParallelize: true,
		},
		{
			plan: planComponents{main: planMaybePhysical{planNode: &renderNode{
				source: planDataSource{plan: scanToParallelize},
				render: []tree.TypedExpr{&tree.IsNullExpr{}},
			}}},
			// Not a simple projection (some expressions might be handled by
			// wrapping a row-execution processor, so we choose to be safe and
			// prohibit the parallelization for all non-IndexedVar expressions).
			prohibitParallelization: true,
		},
		{
			plan:                     planComponents{main: planMaybePhysical{planNode: &sortNode{plan: scanToParallelize}}},
			hasScanNodeToParallelize: true,
		},
		{
			plan:                     planComponents{main: planMaybePhysical{planNode: &unionNode{left: scanToParallelize, right: &scanNode{}}}},
			hasScanNodeToParallelize: true,
		},
		{
			plan:                     planComponents{main: planMaybePhysical{planNode: &unionNode{right: scanToParallelize, left: &scanNode{}}}},
			hasScanNodeToParallelize: true,
		},
		{
			plan:                     planComponents{main: planMaybePhysical{planNode: &valuesNode{}}},
			hasScanNodeToParallelize: false,
		},
		{
			plan: planComponents{main: planMaybePhysical{planNode: &windowNode{plan: scanToParallelize}}},
			// windowNode is not fully supported by the vectorized.
			prohibitParallelization: true,
		},

		// Unsupported edge cases.
		{
			plan:                    planComponents{main: planMaybePhysical{physPlan: &physicalPlanTop{}}},
			prohibitParallelization: true,
		},
		{
			plan:                    planComponents{cascades: []cascadeMetadata{{}}},
			prohibitParallelization: true,
		},
		{
			plan:                    planComponents{checkPlans: []checkPlan{{}}},
			prohibitParallelization: true,
		},
	} {
		prohibitParallelization, hasScanNodeToParallize := checkScanParallelizationIfLocal(context.Background(), &tc.plan)
		require.Equal(t, tc.prohibitParallelization, prohibitParallelization)
		require.Equal(t, tc.hasScanNodeToParallelize, hasScanNodeToParallize)
	}
}

type mockAddressResolver map[base.SQLInstanceID]roachpb.Locality

var _ sqlinstance.AddressResolver = mockAddressResolver{}

func (m mockAddressResolver) GetInstance(
	_ context.Context, id base.SQLInstanceID,
) (sqlinstance.InstanceInfo, error) {
	return sqlinstance.InstanceInfo{InstanceID: id, Locality: m[id]}, nil
}

func (m mockAddressResolver) GetAllInstances(
	_ context.Context,
) ([]sqlinstance.InstanceInfo, error) {
	res := make([]sqlinstance.InstanceInfo, 0, len(m))
	for i := range m {
		res = append(res, sqlinstance.InstanceInfo{InstanceID: i, Locality: m[i]})
	}
	return res, nil
}

func TestClosestInstances(t *testing.T) {
	defer leaktest.AfterTest(t)()
	type instances map[int]string
	type picked []int

	for _, tc := range []struct {
		instances                instances
		loc                      string
		expected                 []int
		expectedLocalityStrength int
	}{
		{instances{1: "a=x", 2: "a=y", 3: "a=z"}, "z=z", picked{}, 0},
		{instances{1: "a=x", 2: "a=y", 3: "a=z"}, "", picked{}, 0},

		{instances{1: "a=x", 2: "a=y", 3: "a=z"}, "a=x", picked{1}, 1},
		{instances{1: "a=x", 2: "a=y", 3: "a=z"}, "a=z", picked{3}, 1},
		{instances{1: "a=x", 2: "a=x", 3: "a=z", 4: "a=z"}, "a=x", picked{1, 2}, 1},
		{instances{1: "a=x", 2: "a=x", 3: "a=z", 4: "a=z"}, "a=z", picked{3, 4}, 1},

		{instances{1: "a=x,b=1", 2: "a=x,b=2", 3: "a=x,b=3", 4: "a=y,b=1", 5: "a=z,b=1"}, "a=x",
			picked{1, 2, 3}, 1},
		{instances{1: "a=x,b=1", 2: "a=x,b=2", 3: "a=x,b=3", 4: "a=y,b=1", 5: "a=z,b=1"}, "a=x,b=2",
			picked{2}, 2},
		{instances{1: "a=x,b=1", 2: "a=x,b=2", 3: "a=x,b=3", 4: "a=y,b=1", 5: "a=z,b=1"}, "a=z",
			picked{5}, 1},
	} {
		t.Run("", func(t *testing.T) {
			var l roachpb.Locality
			if tc.loc != "" {
				require.NoError(t, l.Set(tc.loc))
			}
			var infos []sqlinstance.InstanceInfo
			for id, l := range tc.instances {
				info := sqlinstance.InstanceInfo{InstanceID: base.SQLInstanceID(id)}
				if l != "" {
					require.NoError(t, info.Locality.Set(l))
				}
				infos = append(infos, info)
			}
			var got picked
			instances, strength := ClosestInstances(infos, l)
			for _, i := range instances {
				got = append(got, int(i))
			}
			require.ElementsMatch(t, tc.expected, got)
			require.Equal(t, tc.expectedLocalityStrength, strength)
		})
	}
}
