// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package opt

import (
	"context"
	"math/rand"

	"github.com/cockroachdb/cockroach/pkg/sql/sem/eval"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/errors"
)

const panicInjectionProbability = 0.0005

// MaybeInjectOptimizerTestingPanic has a small chance of creating a panic. This
// is used to test that the optimizer panic-catching error propagation is
// correctly set up in all cases. This is only done if TestingOptimizerInjectPanics
// is enabled.
func MaybeInjectOptimizerTestingPanic(ctx context.Context, evalCtx *eval.Context) {
	if evalCtx.SessionData().TestingOptimizerInjectPanics {
		if rand.Float64() < panicInjectionProbability {
			log.Info(ctx, "injecting panic in optimizer")
			panic(errors.AssertionFailedf("injected panic in optimizer"))
		}
	}
}
