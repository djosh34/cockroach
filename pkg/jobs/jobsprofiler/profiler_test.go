// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package jobsprofiler_test

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/jobs"
	"github.com/cockroachdb/cockroach/pkg/jobs/jobspb"
	"github.com/cockroachdb/cockroach/pkg/jobs/jobsprofiler"
	"github.com/cockroachdb/cockroach/pkg/jobs/jobsprofiler/profilerconstants"
	"github.com/cockroachdb/cockroach/pkg/sql"
	"github.com/cockroachdb/cockroach/pkg/sql/execinfrapb"
	"github.com/cockroachdb/cockroach/pkg/sql/isql"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/uuid"
	"github.com/stretchr/testify/require"
)

func TestStorePerNodeProcessorProgressFraction(t *testing.T) {
	defer leaktest.AfterTest(t)()
	defer log.Scope(t).Close(t)

	s := serverutils.StartServerOnly(t, base.TestServerArgs{
		Knobs: base.TestingKnobs{
			JobsTestingKnobs: jobs.NewTestingKnobsWithShortIntervals(),
		},
	})
	ctx := context.Background()
	defer s.Stopper().Stop(ctx)

	randID := uuid.FastMakeV4()
	componentID := execinfrapb.ComponentID{
		FlowID: execinfrapb.FlowID{UUID: randID},
		Type:   execinfrapb.ComponentID_PROCESSOR,
	}

	jobID := jobspb.JobID(42)

	// Record progress for two components.
	n1proc1 := componentID
	n1proc1.SQLInstanceID = 1
	n1proc1.ID = 1

	n2proc1 := componentID
	n2proc1.SQLInstanceID = 2
	n2proc1.ID = 1

	jobsprofiler.StorePerNodeProcessorProgressFraction(ctx, s.InternalDB().(isql.DB),
		jobID, map[execinfrapb.ComponentID]float32{n1proc1: 0.95, n2proc1: 0.50})

	// Update n2proc1.
	jobsprofiler.StorePerNodeProcessorProgressFraction(ctx, s.InternalDB().(isql.DB),
		jobID, map[execinfrapb.ComponentID]float32{n2proc1: 0.70})
	// Update n1proc1.
	jobsprofiler.StorePerNodeProcessorProgressFraction(ctx, s.InternalDB().(isql.DB),
		jobID, map[execinfrapb.ComponentID]float32{n1proc1: 1.00})

	var persistedProgress map[string]string
	err := s.ExecutorConfig().(sql.ExecutorConfig).InternalDB.Txn(ctx, func(ctx context.Context, txn isql.Txn) error {
		persistedProgress = make(map[string]string)
		infoStorage := jobs.InfoStorageForJob(txn, jobID)
		return infoStorage.Iterate(ctx, profilerconstants.NodeProcessorProgressInfoKeyPrefix,
			func(infoKey string, value []byte) error {
				f, err := strconv.ParseFloat(string(value), 32)
				require.NoError(t, err)
				f = math.Ceil(f*100) / 100
				persistedProgress[infoKey] = fmt.Sprintf("%.2f", f)
				return nil
			})
	})
	if err != nil {
		t.Fatal(err)
	}

	require.Len(t, persistedProgress, 2)
	for k, v := range persistedProgress {
		if strings.Contains(k, fmt.Sprintf("%s,1,1", randID.String())) {
			require.Equal(t, "1.00", v)
		} else if strings.Contains(k, fmt.Sprintf("%s,2,1", randID.String())) {
			require.Equal(t, "0.70", v)
		} else {
			t.Fatalf("unexpected info key %s:%s", k, v)
		}
	}
}
