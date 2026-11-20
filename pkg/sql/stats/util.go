// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package stats

import (
	"math/rand"

	"github.com/cockroachdb/cockroach/pkg/sql/catalog/descpb"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
	"github.com/cockroachdb/cockroach/pkg/util/intsets"
)

// MakeSortedColStatKey constructs a unique key representing cols that can be
// used as the key in a map, and also sorts cols as a side-effect.
func MakeSortedColStatKey(cols []descpb.ColumnID) string {
	var colSet intsets.Fast
	for _, c := range cols {
		colSet.Add(int(c))
	}
	// We've already done the work to order the column set, so might as well make
	// cols match that ordering now instead of sorting it later.
	var i int
	colSet.ForEach(func(c int) {
		cols[i] = descpb.ColumnID(c)
		i++
	})
	return colSet.String()
}

// These two are used only by tests and are defined to prevent an import cycle.
var (
	// RandType is randgen.RandType.
	RandType func(rng *rand.Rand) *types.T
	// RandDatum is randgen.RandDatum.
	RandDatum func(rng *rand.Rand, typ *types.T, nullOk bool) tree.Datum
)
