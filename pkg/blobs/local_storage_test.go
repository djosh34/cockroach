// Copyright 2019 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package blobs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/util/leaktest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDirectoryNormalization(t *testing.T) {
	l, err := NewLocalStorage("././.")
	if err != nil {
		t.Fatal(err)
	}
	expected, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, expected, l.externalIODir)
}

// TestSiblingDirectoryIsOutside checks that a directory whose name starts with
// the name of the external I/O directory is not inside it.
func TestSiblingDirectoryIsOutside(t *testing.T) {
	defer leaktest.AfterTest(t)()
	parent := t.TempDir()
	ioDir := filepath.Join(parent, "extern")
	require.NoError(t, os.MkdirAll(filepath.Join(parent, "extern-secret"), 0755))
	require.NoError(t, os.MkdirAll(ioDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(parent, "extern-secret", "f"), []byte("secret"), 0644))

	l, err := NewLocalStorage(ioDir)
	require.NoError(t, err)
	_, err = l.prependExternalIODir("../extern-secret/f")
	require.ErrorContains(t, err, "outside of external-io-dir")

	p, err := l.prependExternalIODir("a/b")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(ioDir, "a", "b"), p)
	_, err = l.prependExternalIODir("")
	require.NoError(t, err)
}
