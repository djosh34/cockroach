// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package ptutil

import (
	"context"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/protectedts/ptpb"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/spanconfig"
	"github.com/cockroachdb/cockroach/pkg/spanconfig/spanconfigptsreader"
	"github.com/cockroachdb/cockroach/pkg/testutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/sqlutils"
	"github.com/cockroachdb/cockroach/pkg/util/hlc"
	"github.com/cockroachdb/cockroach/pkg/util/protoutil"
	"github.com/cockroachdb/cockroach/pkg/util/uuid"
	"github.com/cockroachdb/errors"
)

// TestingWaitForProtectedTimestampToExistOnSpans waits
// testutils.SucceedsSoonDuration for the supplied protected timestamp to exist
// on all the supplied spans. It fatals if this doesn't happen in time.
func TestingWaitForProtectedTimestampToExistOnSpans(
	ctx context.Context,
	t *testing.T,
	srv serverutils.TestServerInterface,
	ptsReader spanconfig.ProtectedTSReader,
	protectedTimestamp hlc.Timestamp,
	spans roachpb.Spans,
) {
	testutils.SucceedsSoon(t, func() error {
		if err := spanconfigptsreader.TestingRefreshPTSState(
			ctx, t, ptsReader, srv.Clock().Now(),
		); err != nil {
			return err
		}
		for _, sp := range spans {
			timestamps, _, err := ptsReader.GetProtectionTimestamps(ctx, sp)
			if err != nil {
				return err
			}
			found := false
			for _, ts := range timestamps {
				if ts.Equal(protectedTimestamp) {
					found = true
					break
				}
			}
			if !found {
				return errors.Newf("protection timestamp %s does not exist on span %s", protectedTimestamp, sp)
			}
		}
		return nil
	})
}

func GetPTSTarget(t *testing.T, db *sqlutils.SQLRunner, ptsID *uuid.UUID) *ptpb.Target {
	ret := &ptpb.Target{}
	var buf []byte
	db.QueryRow(t, `SELECT target FROM system.protected_ts_records WHERE id = $1`, ptsID).Scan(&buf)
	if err := protoutil.Unmarshal(buf, ret); err != nil {
		t.Fatal(err)
	}
	return ret
}
