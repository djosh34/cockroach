// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.
//

package echotest

import (
	"fmt"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/testutils/datapathutils"
)

func TestWalk(t *testing.T) {
	w := NewWalker(t, datapathutils.TestDataPath(t))

	// NB: a desirable property here is that in Goland you can click on the
	// subtest to navigate to the entry in the slice. This requires that `t.Run`
	// is invoked directly with the test name (this is a trial and error finding).
	for _, test := range []struct{ name string }{
		{name: "foo"},
		{name: "bar"},
	} {
		t.Run(test.name, w.Run(t, test.name, func(t *testing.T) string {
			return fmt.Sprintf("hello, %s", test.name)
		}))
	}
}
