// Copyright 2016 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package testutils

import (
	"os"

	"github.com/cockroachdb/cockroach/pkg/util/fileutil"
)

// TempDir creates a directory and a function to clean it up at the end of the
// test.
func TempDir(t TestNamedFatalerLogger) (string, func()) {
	dir, err := os.MkdirTemp("", fileutil.EscapeFilename(t.Name()))
	if err != nil {
		t.Fatal(err)
	}
	cleanup := func() {
		if err := os.RemoveAll(dir); err != nil {
			// Temp dir cleanup may race with process shutdown, where
			// some process is writing to dir, just as we try to shut down.
			// This should not fail the test.
			t.Logf("encountered error %s while removing temp dir %s", err, dir)
		}
	}
	return dir, cleanup
}
