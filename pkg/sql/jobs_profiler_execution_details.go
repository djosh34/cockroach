// Copyright 2023 The Cockroach Authors.
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
	gojson "encoding/json"

	"github.com/cockroachdb/cockroach/pkg/clusterversion"
	"github.com/cockroachdb/cockroach/pkg/jobs"
	"github.com/cockroachdb/cockroach/pkg/jobs/jobspb"
	"github.com/cockroachdb/cockroach/pkg/jobs/jobsprofiler/profilerconstants"
	"github.com/cockroachdb/cockroach/pkg/server/serverpb"
	"github.com/cockroachdb/cockroach/pkg/sql/isql"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/eval"
	"github.com/cockroachdb/errors"
)

// GenerateExecutionDetailsJSON implements the Profiler interface.
func (p *planner) GenerateExecutionDetailsJSON(
	ctx context.Context, evalCtx *eval.Context, jobID jobspb.JobID,
) ([]byte, error) {
	execCfg := evalCtx.Planner.ExecutorConfig().(*ExecutorConfig)
	if _, err := execCfg.JobRegistry.LoadJob(ctx, jobID); err != nil {
		return nil, err
	}
	return constructDefaultExecutionDetails(ctx, jobID, execCfg.InternalDB)
}

// defaultExecutionDetails is a JSON serializable struct that captures the
// execution details that are not specific to a particular job type.
type defaultExecutionDetails struct {
	// PlanDiagram is the URL to render the latest DistSQL plan that is being
	// executed by the job.
	PlanDiagram string `json:"plan_diagram"`
}

func constructDefaultExecutionDetails(
	ctx context.Context, jobID jobspb.JobID, db isql.DB,
) ([]byte, error) {
	executionDetails := &defaultExecutionDetails{}
	err := db.Txn(ctx, func(ctx context.Context, txn isql.Txn) error {
		// Populate the latest DSP diagram URL.
		infoStorage := jobs.InfoStorageForJob(txn, jobID)
		err := infoStorage.GetLast(ctx, profilerconstants.DSPDiagramInfoKeyPrefix, func(infoKey string, value []byte) error {
			executionDetails.PlanDiagram = string(value)
			return nil
		})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	j, err := gojson.Marshal(executionDetails)
	return j, err
}

// RequestExecutionDetailFiles implements the JobProfiler interface.
func (p *planner) RequestExecutionDetailFiles(ctx context.Context, jobID jobspb.JobID) error {
	execCfg := p.ExecCfg()
	if !execCfg.Settings.Version.IsActive(ctx, clusterversion.V23_2) {
		return errors.Newf("execution details can only be requested on a cluster with version >= %s",
			clusterversion.V23_2.String())
	}

	_, err := execCfg.SQLStatusServer.RequestJobProfilerExecutionDetails(ctx,
		&serverpb.RequestJobProfilerExecutionDetailsRequest{
			JobId: int64(jobID),
		})
	return err
}
