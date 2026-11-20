// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package randgen

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/sql/catalog/randgen/randgencfg"
	"github.com/cockroachdb/cockroach/pkg/util/randident"
)

// TestSchemaGeneratorConfig is an alias for randgencfg.Config.
type TestSchemaGeneratorConfig = randgencfg.Config

// TestSchemaGenerator can be used to generate random SQL objects.
// See NewTestSchemaGenerator().
//
// A design goal for this facility is to ensure that the generation is
// fast in the common case, so it can be used to generate large
// schemas quickly.
type TestSchemaGenerator interface {
	// Generate produces SQL test objects.
	Generate(ctx context.Context) (finalCfg TestSchemaGeneratorConfig, err error)
}

// DefaultTestSchemaGeneratorConfig produces a default configuration
// for the test schema generator.
func DefaultTestSchemaGeneratorConfig() TestSchemaGeneratorConfig {
	return TestSchemaGeneratorConfig{
		// The following default batch size was guesstimated. More
		// experiments needed to determine whether this is a good default.
		// A small value (10-ish or smaller) definitely delivers poor
		// performance.
		BatchSize: 1000,

		// Let's have a different random generation every time.
		Seed: nil,

		// Ensure that the tables look a bit different.
		RandomizeColumns: true,

		// Do some work by default.
		Counts: []int{10},

		// Automatically assign names from template(s).
		Names: "_",

		NameGen: randident.DefaultNameGeneratorConfig(),
	}
}
