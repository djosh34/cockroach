// Copyright 2019 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

// Command roachvet is a vettool which includes all of the standard analysis
// passes included in go vet as well as the `shadow` pass and some first-party
// passes.
package main

import (
	"fmt"
	"os"

	"github.com/cockroachdb/cockroach/pkg/testutils/lint/passes/deferunlockcheck"
	"github.com/cockroachdb/cockroach/pkg/testutils/lint/passes/errcmp"
	"github.com/cockroachdb/cockroach/pkg/testutils/lint/passes/errwrap"
	"github.com/cockroachdb/cockroach/pkg/testutils/lint/passes/fmtsafe"
	"github.com/cockroachdb/cockroach/pkg/testutils/lint/passes/forbiddenmethod"
	"github.com/cockroachdb/cockroach/pkg/testutils/lint/passes/hash"
	"github.com/cockroachdb/cockroach/pkg/testutils/lint/passes/leaktestcall"
	"github.com/cockroachdb/cockroach/pkg/testutils/lint/passes/loopvarcapture"
	"github.com/cockroachdb/cockroach/pkg/testutils/lint/passes/nilness"
	"github.com/cockroachdb/cockroach/pkg/testutils/lint/passes/nocopy"
	"github.com/cockroachdb/cockroach/pkg/testutils/lint/passes/redactcheck"
	"github.com/cockroachdb/cockroach/pkg/testutils/lint/passes/returnerrcheck"
	"github.com/cockroachdb/cockroach/pkg/testutils/lint/passes/timer"
	"github.com/cockroachdb/cockroach/pkg/testutils/lint/passes/unconvert"
	"github.com/kisielk/errcheck/errcheck"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/asmdecl"
	"golang.org/x/tools/go/analysis/passes/assign"
	"golang.org/x/tools/go/analysis/passes/atomic"
	"golang.org/x/tools/go/analysis/passes/bools"
	"golang.org/x/tools/go/analysis/passes/buildtag"
	"golang.org/x/tools/go/analysis/passes/cgocall"
	"golang.org/x/tools/go/analysis/passes/composite"
	"golang.org/x/tools/go/analysis/passes/copylock"
	"golang.org/x/tools/go/analysis/passes/errorsas"
	"golang.org/x/tools/go/analysis/passes/httpresponse"
	"golang.org/x/tools/go/analysis/passes/lostcancel"
	"golang.org/x/tools/go/analysis/passes/nilfunc"
	"golang.org/x/tools/go/analysis/passes/printf"
	"golang.org/x/tools/go/analysis/passes/shadow"
	"golang.org/x/tools/go/analysis/passes/shift"
	"golang.org/x/tools/go/analysis/passes/stdmethods"
	"golang.org/x/tools/go/analysis/passes/structtag"
	"golang.org/x/tools/go/analysis/passes/tests"
	"golang.org/x/tools/go/analysis/passes/unmarshal"
	"golang.org/x/tools/go/analysis/passes/unreachable"
	"golang.org/x/tools/go/analysis/passes/unsafeptr"
	"golang.org/x/tools/go/analysis/passes/unusedresult"
	"golang.org/x/tools/go/analysis/unitchecker"
)

func main() {
	var as []*analysis.Analyzer
	// First-party analyzers:
	as = append(as, forbiddenmethod.Analyzers...)
	as = append(as,
		hash.Analyzer,
		leaktestcall.Analyzer,
		nocopy.Analyzer,
		redactcheck.Analyzer,
		returnerrcheck.Analyzer,
		timer.Analyzer,
		unconvert.Analyzer,
		fmtsafe.Analyzer,
		errcmp.Analyzer,
		nilness.Analyzer,
		errwrap.Analyzer,
		loopvarcapture.Analyzer,
		deferunlockcheck.Analyzer,
	)

	// Standard go vet analyzers:
	as = append(as,
		asmdecl.Analyzer,
		assign.Analyzer,
		atomic.Analyzer,
		bools.Analyzer,
		buildtag.Analyzer,
		cgocall.Analyzer,
		composite.Analyzer,
		copylock.Analyzer,
		errorsas.Analyzer,
		httpresponse.Analyzer,
		// loopclosure.Analyzer,
		// loopclosure is superseded by 'loopvarcapture'
		lostcancel.Analyzer,
		nilfunc.Analyzer,
		printf.Analyzer,
		shift.Analyzer,
		stdmethods.Analyzer,
		structtag.Analyzer,
		tests.Analyzer,
		unmarshal.Analyzer,
		unreachable.Analyzer,
		unsafeptr.Analyzer,
		unusedresult.Analyzer,
	)

	// Additional analyzers:
	as = append(as,
		shadow.Analyzer,
		errcheck.Analyzer,
	)

	// ROACHVET_PART=K/N runs only every Nth analyzer, starting at the Kth.
	// CI spreads the analyzers over several jobs this way.
	if part := os.Getenv("ROACHVET_PART"); part != "" {
		var k, n int
		if _, err := fmt.Sscanf(part, "%d/%d", &k, &n); err != nil || n <= 0 || k < 0 || k >= n {
			fmt.Fprintf(os.Stderr, "roachvet: invalid ROACHVET_PART %q, want K/N with 0 <= K < N\n", part)
			os.Exit(2)
		}
		var mine []*analysis.Analyzer
		for i, a := range as {
			if i%n == k {
				mine = append(mine, a)
			}
		}
		as = mine
	}

	unitchecker.Main(as...)
}
