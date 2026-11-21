// Copyright 2026 The Cockroach Authors.
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

	"github.com/cockroachdb/cockroach/pkg/jobs"
	"github.com/cockroachdb/cockroach/pkg/jobs/jobspb"
	"github.com/cockroachdb/cockroach/pkg/util/hlc"
	"github.com/cockroachdb/errors"
)

// RestoreCluster is part of the eval.Planner interface. The job is in
// pkg/backup.
func (p *planner) RestoreCluster(
	ctx context.Context, bucket, prefix, endpoint, region, target string,
) (int64, error) {
	if bucket == "" {
		return 0, errors.New("the bucket is required")
	}
	details := jobspb.ClusterRestoreDetails{
		Bucket: bucket, Prefix: prefix, Endpoint: endpoint, Region: region,
	}
	if target != "" {
		ts, err := hlc.ParseHLC(target)
		if err != nil {
			return 0, errors.Wrapf(err, "parsing the target %q", target)
		}
		details.Target = ts
	}
	record := jobs.Record{
		JobID:       p.ExecCfg().JobRegistry.MakeJobID(),
		Description: fmt.Sprintf("restore the cluster from s3://%s/%s", bucket, prefix),
		Username:    p.User(),
		Details:     details,
		Progress:    jobspb.ClusterRestoreProgress{},
	}
	p.extendedEvalCtx.jobs.addNonUniqueJobToCreate(&record)
	return int64(record.JobID), nil
}
