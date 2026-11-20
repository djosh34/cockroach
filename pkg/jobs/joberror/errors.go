// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package joberror

import (
	"github.com/cockroachdb/cockroach/pkg/kv/kvclient/kvcoord"
	"github.com/cockroachdb/cockroach/pkg/sql/flowinfra"
	"github.com/cockroachdb/cockroach/pkg/sql/sqlerrors"
	"github.com/cockroachdb/cockroach/pkg/sql/sqlinstance"
	"github.com/cockroachdb/cockroach/pkg/util/circuit"
	"github.com/cockroachdb/cockroach/pkg/util/grpcutil"
	"github.com/cockroachdb/cockroach/pkg/util/sysutil"
	"github.com/cockroachdb/errors"
)

// IsPermanentBulkJobError returns true if the error results in a permanent
// failure of a bulk job (IMPORT, BACKUP, RESTORE). This function is an
// allowlist instead of a blocklist: only known safe errors are confirmed to not
// be permanent errors. Anything unknown is assumed to be permanent.
func IsPermanentBulkJobError(err error) bool {
	if err == nil {
		return false
	}
	return !sqlerrors.IsDistSQLRetryableError(err) &&
		!grpcutil.IsClosedConnection(err) &&
		!flowinfra.IsFlowRetryableError(err) &&
		!flowinfra.IsNoInboundStreamConnectionError(err) &&
		!kvcoord.IsSendError(err) &&
		!errors.Is(err, circuit.ErrBreakerOpen) &&
		!sysutil.IsErrConnectionReset(err) &&
		!sysutil.IsErrConnectionRefused(err) &&
		!errors.Is(err, sqlinstance.NonExistentInstanceError)
}
