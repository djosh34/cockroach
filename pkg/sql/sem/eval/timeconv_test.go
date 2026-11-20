// Copyright 2017 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package eval_test

import (
	"context"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/kv"
	"github.com/cockroachdb/cockroach/pkg/kv/kvpb"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/concurrency/isolation"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	_ "github.com/cockroachdb/cockroach/pkg/sql/sem/builtins"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/eval"
	"github.com/cockroachdb/cockroach/pkg/util/hlc"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/stop"
	"github.com/stretchr/testify/require"
)

// Test that EvalContext.GetClusterTimestamp() gets its timestamp from the
// transaction, and also that the conversion to decimal works properly.
func TestClusterTimestampConversion(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)
	testData := []struct {
		walltime int64
		logical  int32
		expected string
	}{
		{42, 0, "42.0000000000"},
		{-42, 0, "-42.0000000000"},
		{42, 69, "42.0000000069"},
		{42, 2147483647, "42.2147483647"},
		{9223372036854775807, 2147483647, "9223372036854775807.2147483647"},
	}

	ctx := context.Background()
	stopper := stop.NewStopper()
	defer stopper.Stop(ctx)

	clock := hlc.NewClockForTesting(nil)
	senderFactory := kv.MakeMockTxnSenderFactory(
		func(context.Context, *roachpb.Transaction, *kvpb.BatchRequest,
		) (*kvpb.BatchResponse, *kvpb.Error) {
			panic("unused")
		})
	db := kv.NewDB(log.MakeTestingAmbientCtxWithNewTracer(), senderFactory, clock, stopper)

	for _, d := range testData {
		ts := hlc.ClockTimestamp{WallTime: d.walltime, Logical: d.logical}
		txnProto := roachpb.MakeTransaction(
			"test",
			nil, // baseKey
			isolation.Serializable,
			roachpb.NormalUserPriority,
			ts.ToTimestamp(),
			0, // maxOffsetNs
			1, // coordinatorNodeID
			0,
			false, // omitInRangefeeds
		)

		ctx := eval.Context{
			Txn: kv.NewTxnFromProto(
				context.Background(),
				db,
				1, /* gatewayNodeID */
				ts,
				kv.RootTxn,
				&txnProto,
			),
		}

		dec, err := ctx.GetClusterTimestamp()
		require.NoError(t, err)
		final := dec.Text('f')
		if final != d.expected {
			t.Errorf("expected %s, but found %s", d.expected, final)
		}
	}
}
