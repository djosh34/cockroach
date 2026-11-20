// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package application_api_test

import (
	"github.com/cockroachdb/cockroach/pkg/util/randident"
	"github.com/cockroachdb/cockroach/pkg/util/randutil"
)

func generateRandomName() string {
	rand, _ := randutil.NewTestRand()
	cfg := randident.DefaultNameGeneratorConfig()
	// REST api can not handle `/`. This is fixed in
	// the UI by using sql-over-http endpoint instead.
	cfg.Punctuate = -1
	cfg.Finalize()

	ng := randident.NewNameGenerator(
		&cfg,
		rand,
		"a b%s-c.d",
	)
	return ng.GenerateOne(42)
}
