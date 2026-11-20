// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package metamorphic

import (
	"os"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/testutils/datapathutils"
	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/stretchr/testify/require"
)

func TestParseOutputPreamble(t *testing.T) {
	defer leaktest.AfterTest(t)()

	f, err := os.Open(datapathutils.TestDataPath(t, "sample.meta"))
	require.NoError(t, err)

	cfg, seed, err := parseOutputPreamble(f)
	require.NoError(t, err)
	require.Equal(t, seed, int64(7375396416917217630))
	require.Equal(t, cfg.name, "random-007")
	// TODO(jackson): Assert roundtrip equality.
	t.Log(cfg.opts.EnsureDefaults().String())
}
