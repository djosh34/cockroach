// Copyright 2017 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package builtins

import (
	"regexp"
	"strings"
	"testing"
	"unicode"

	"github.com/cockroachdb/cockroach/pkg/sql/parser"
	"github.com/cockroachdb/cockroach/pkg/sql/pgwire/pgerror"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/builtins/builtinconstants"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/builtins/builtinsregistry"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
)

func TestHelpFunctions(t *testing.T) {
	defer leaktest.AfterTest(t)()
	numTestsRun := 0
	// This test checks that all the built-in functions receive contextual help.
	builtinsregistry.AddSubscription(func(f string, prop *tree.FunctionProperties, _ []tree.Overload) {
		if unicode.IsUpper(rune(f[0])) {
			return
		}
		if prop.Category == builtinconstants.CategoryCast {
			return
		}
		t.Run(f, func(t *testing.T) {
			numTestsRun++
			_, err := parser.Parse("select " + f + "(??")
			if err == nil {
				t.Errorf("parser didn't trigger error")
				return
			}
			if !strings.HasPrefix(err.Error(), "help token in input") {
				t.Fatal(err)
			}
			pgerr := pgerror.Flatten(err)
			if !strings.HasPrefix(pgerr.Hint, "help:\n") {
				t.Errorf("expected 'help: ' prefix, got %q", pgerr.Hint)
				return
			}
			help := pgerr.Hint[6:]
			pattern := "Function:\\s+" + f + "\n"
			if m, err := regexp.MatchString(pattern, help); err != nil || !m {
				if err != nil {
					t.Errorf("pattern match failure: %v", err)
					return
				}
				t.Errorf("help text didn't match %q:\n%s", pattern, help)
			}
		})
	})

	// About 770 builtins remain after the spatial functions were removed.
	if numTestsRun < 700 {
		t.Errorf("Test saw %d builtins, probably load order is wrong", numTestsRun)
	}
}
