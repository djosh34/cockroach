// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package upgrades

import (
	"context"
	"fmt"

	"github.com/cockroachdb/cockroach/pkg/clusterversion"
	"github.com/cockroachdb/cockroach/pkg/security/username"
	"github.com/cockroachdb/cockroach/pkg/sql/sessiondata"
	"github.com/cockroachdb/cockroach/pkg/upgrade"
)

func sqlStatsTTLChange(
	ctx context.Context, _ clusterversion.ClusterVersion, d upgrade.SystemDeps,
) error {
	tables := []string{
		"system.statement_statistics",
		"system.transaction_statistics",
		"system.statement_activity",
		"system.transaction_activity",
	}

	// These migrations are skipped for backup tests because these tables are not part
	// of the backup.
	shouldConfigureTTL := true
	if knobs := d.SQLStatsKnobs; knobs != nil {
		shouldConfigureTTL = !knobs.SkipZoneConfigBootstrap
	}

	if shouldConfigureTTL {
		for _, table := range tables {
			if _, err := d.DB.Executor().ExecEx(
				ctx,
				"set-SQLStatsTables-TTL",
				nil,
				sessiondata.InternalExecutorOverride{User: username.RootUserName()},
				fmt.Sprintf("ALTER TABLE %s CONFIGURE ZONE USING gc.ttlseconds = $1", table),
				3600, /* one hour */
			); err != nil {
				return err
			}
		}
	}
	return nil
}
