// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package keyvisstorage

import (
	"context"
	"fmt"
	"time"

	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/cockroach/pkg/sql"
	"github.com/cockroachdb/cockroach/pkg/sql/sessiondata"
)

// DeleteSamplesBeforeTime deletes collected samples that were taken before time.
func DeleteSamplesBeforeTime(ctx context.Context, ie *sql.InternalExecutor, t time.Time) error {

	// Delete samples and sample buckets that have expired.
	stmt := fmt.Sprintf(
		"WITH "+
			"	deleted_rows AS ("+
			"		DELETE FROM system.span_stats_samples "+
			"		WHERE sample_time < CAST(%d AS TIMESTAMP)"+
			"		RETURNING id"+
			"	)"+
			"DELETE FROM system.span_stats_buckets "+
			"WHERE sample_id IN (SELECT id FROM deleted_rows)",
		t.Unix())

	_, err := ie.ExecEx(ctx, "delete-expired-samples", nil,
		sessiondata.InternalExecutorOverride{User: username.RootUserName()}, stmt)

	if err != nil {
		return err
	}

	// Delete keys that are no longer referenced by any buckets.
	deleteKeysStmt := "" +
		"DELETE FROM system.span_stats_unique_keys " +
		"WHERE " +
		"	NOT EXISTS (" +
		"		SELECT * " +
		"		FROM system.span_stats_buckets " +
		"		WHERE " +
		"			system.span_stats_buckets.start_key_id = system.span_stats_unique_keys.id " +
		"			OR system.span_stats_buckets.end_key_id = system.span_stats_unique_keys.id" +
		"	)"

	_, err = ie.ExecEx(
		ctx,
		"delete-unused-start-keys",
		nil,
		sessiondata.InternalExecutorOverride{User: username.RootUserName()},
		deleteKeysStmt,
	)
	return err
}
