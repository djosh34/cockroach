// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package cast

import (
	"strings"

	"github.com/cockroachdb/cockroach/pkg/sql/types"
	"github.com/lib/pq/oid"
)

// CastTypeName returns the name of the type used for casting.
func CastTypeName(t *types.T) string {
	// SQLString is wrong for these types.
	switch t.Oid() {
	case oid.T_numeric:
		// SQLString returns `decimal`
		return "numeric"
	case oid.T_char:
		// SQLString returns `"char"`
		return "char"
	case oid.T_bpchar:
		// SQLString returns `char`.
		return "bpchar"
	case oid.T_text:
		// SQLString returns `string`
		return "text"
	}
	return strings.ToLower(t.SQLString())
}
