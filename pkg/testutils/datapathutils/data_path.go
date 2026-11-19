// Copyright 2020 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package datapathutils

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// DebuggableTempDir returns a temporary directory that is appropriate for
// retaining leftover artifacts in case the temp fails for debugging. It
// returns os.TempDir(): the person executing the test should set up their
// $TMPDIR appropriately so they know where to find these leftover artifacts.
// This function is specifically for when we want to capture left-over artifacts
// after the test completes; if you are always going to clean up the files you
// create either way, testutils.TempDir() may be more convenient.
func DebuggableTempDir() string {
	return os.TempDir()
}

// TestDataPath returns a path to an asset in the testdata directory of the
// package under test.
//
// For example, if there is a file testdata/a.txt, you can get a path to that
// file using TestDataPath(t, "a.txt").
func TestDataPath(t testing.TB, relative ...string) string {
	relative = append([]string{"testdata"}, relative...)
	// Tests run in the package directory, so a relative path works.
	ret := path.Join(relative...)
	ret, err := filepath.Abs(ret)
	require.NoError(t, err)
	return ret
}

// RewritableDataPath returns a path to an asset relative to the top of the
// workspace. Generally you should use TestDataPath if you're trying to access
// a file in your test's `testdata` directory. This function is only necessary
// if you need the path to a file outside the package under test.
func RewritableDataPath(t testing.TB, relative ...string) string {
	// Get the path to this file from the runtime.
	_, thisFilePath, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("unable to get caller information")
	}
	cockroachWorkspace := thisFilePath
	for {
		dir := filepath.Base(cockroachWorkspace)
		cockroachWorkspace = filepath.Dir(cockroachWorkspace)
		const target = "pkg"
		if dir == target {
			break
		}
		if len(cockroachWorkspace) < 2 {
			panic(fmt.Errorf("did not find %q subdirectory in %q", target, thisFilePath))
		}
	}
	relative = append([]string{cockroachWorkspace}, relative...)
	return filepath.Join(relative...)
}
