// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package lsnutil

import (
	"time"

	"github.com/cockroachdb/cockroach/pkg/sql/pgrepl/lsn"
	"github.com/cockroachdb/cockroach/pkg/util/hlc"
)

// HLCToLSN converts a HLC to a LSN.
// It is in a separate package to prevent the `lsn` package importing `log`.
func HLCToLSN(h hlc.Timestamp) lsn.LSN {
	// TODO(#105130): correctly populate this field.
	return lsn.LSN(h.WallTime/int64(time.Millisecond)) << 32
}
