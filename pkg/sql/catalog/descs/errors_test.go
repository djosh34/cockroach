// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package descs

import (
	"testing"

	"github.com/cockroachdb/cockroach/pkg/sql/catalog/lease"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/cockroachdb/redact"
	"github.com/stretchr/testify/require"
)

func TestTwoVersionInvariantViolationError(t *testing.T) {
	defer leaktest.AfterTest(t)()

	err := &twoVersionInvariantViolationError{ids: []lease.IDVersion{
		{
			Name:    "foo",
			ID:      1,
			Version: 2,
		},
		{
			Name:    "bar",
			ID:      2,
			Version: 3,
		},
	}}
	require.EqualError(t, err, "cannot publish new versions for descriptors: "+
		"[{foo 1 2} {bar 2 3}], old versions still in use")
	require.Equal(t, "cannot publish new versions for descriptors: "+
		"[{‹×› 1 2} {‹×› 2 3}], old versions still in use",
		string(redact.Sprint(err).Redact()))
}
