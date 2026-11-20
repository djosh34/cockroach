// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package loopvarcapture_test

import (
	"testing"

	"github.com/cockroachdb/cockroach/pkg/testutils/datapathutils"
	"github.com/cockroachdb/cockroach/pkg/testutils/lint/passes/loopvarcapture"
	"github.com/cockroachdb/cockroach/pkg/testutils/skip"
	"golang.org/x/tools/go/analysis/analysistest"
)

var extraGoRoutineFunctions = []loopvarcapture.Function{
	{Pkg: "example.org/concurrency", Type: "Group", Name: "Go"}, // test non-pointer receiver
	{Pkg: "example.org/concurrency", Name: "Go"},                // test a package-level function
	{Pkg: "example.org/concurrency", Name: "GoWithError"},       // test a function with a return value
	{Pkg: "p", Type: "Monitor", Name: "Go"},                     // test an interface method
}

func TestAnalyzer(t *testing.T) {
	skip.UnderStress(t)

	testAnalyzer := *loopvarcapture.Analyzer
	originalGoRoutineFunctions := loopvarcapture.GoRoutineFunctions
	loopvarcapture.GoRoutineFunctions = append(originalGoRoutineFunctions, extraGoRoutineFunctions...)
	defer func() { loopvarcapture.GoRoutineFunctions = originalGoRoutineFunctions }()

	testdata := datapathutils.TestDataPath(t)
	analysistest.TestData = func() string { return testdata }
	results := analysistest.Run(t, testdata, &testAnalyzer, "p")
	// Perform a sanity check--we expect analysis results.
	if len(results) == 0 {
		t.Fatal("analysis results are empty")
	}
}
