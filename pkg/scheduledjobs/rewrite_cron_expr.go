// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package scheduledjobs

import (
	"fmt"
	"hash/fnv"
	"math/rand"

	"github.com/cockroachdb/cockroach/pkg/util/uuid"
)

const (
	cronWeekly = "@weekly"
	cronDaily  = "@daily"
	cronHourly = "@hourly"
)

// MaybeRewriteCronExpr is used to rewrite the interval-oriented cron exprs
// into an equivalent frequency interval but with an offset derived from the
// uuid. For a given pair of inputs, the output of this function will always
// be the same. If the input cronExpr is not a special form as denoted by
// the keys of cronExprRewrites, it will be returned unmodified. This rewrite
// occurs in order to uniformly distribute the production of telemetry logs
// over the intended time interval to avoid bursts.
func MaybeRewriteCronExpr(id uuid.UUID, cronExpr string) string {
	if f, ok := cronExprRewrites[cronExpr]; ok {
		hash := fnv.New64a() // arbitrary hash function
		_, _ = hash.Write(id.GetBytes())
		return f(rand.New(rand.NewSource(int64(hash.Sum64()))))
	}
	return cronExpr
}

var cronExprRewrites = map[string]func(r *rand.Rand) string{
	cronWeekly: func(r *rand.Rand) string {
		return fmt.Sprintf("%d %d * * %d", r.Intn(60), r.Intn(23), r.Intn(7))
	},
	cronDaily: func(r *rand.Rand) string {
		return fmt.Sprintf("%d %d * * *", r.Intn(60), r.Intn(23))
	},
	cronHourly: func(r *rand.Rand) string {
		return fmt.Sprintf("%d * * * *", r.Intn(60))
	},
}
