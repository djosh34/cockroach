// Copyright 2019 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package reducesql_test

import (
	"context"
	"flag"
	"strings"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/cmd/reduce/reduce"
	"github.com/cockroachdb/cockroach/pkg/cmd/reduce/reduce/reducesql"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/skip"
)

var printUnknown = flag.Bool("unknown", false, "print unknown types during walk")

func TestReduceSQL(t *testing.T) {
	// These take a bit too long to need to run every time.
	skip.IgnoreLint(t, "unnecessary")
	reducesql.LogUnknown = *printUnknown

	isInterestingSQLWrapper := func(contains string) reduce.InterestingFn {
		return isInterestingSQL(t, contains)
	}

	reduce.Walk(t, "testdata", reducesql.Pretty, isInterestingSQLWrapper, reduce.ModeInteresting,
		nil /* chunkReducer */, reducesql.SQLPasses)
}

func isInterestingSQL(t *testing.T, contains string) reduce.InterestingFn {
	return func(ctx context.Context, f string) (bool, func()) {
		args := base.TestServerArgs{
			Insecure: true,
		}

		serv := serverutils.StartServerOnly(t, args)
		defer serv.Stopper().Stop(ctx)

		db := serv.ApplicationLayer().SQLConn(t)

		_, err := db.Exec(f)
		if err == nil {
			return false, nil
		}
		return strings.Contains(err.Error(), contains), nil
	}
}
