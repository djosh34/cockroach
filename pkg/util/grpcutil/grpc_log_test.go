// Copyright 2017 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package grpcutil

import (
	"math"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/util/log/severity"
	"github.com/stretchr/testify/require"
)

func TestGRPCLogger(t *testing.T) {
	{
		// If input < env var severity, use input.
		l := getGRPCLogger(severity.INFO, "warning", "")
		require.Equal(t, l.sev, severity.INFO)
	}

	{
		// If env var smaller than info, use env var.
		l := getGRPCLogger(severity.ERROR, "info", "")
		require.Equal(t, l.sev, severity.INFO)
	}

	{
		// When threshold is ERROR, should not log WARNING or below.
		l := getGRPCLogger(severity.ERROR, "", "")
		require.False(t, l.shouldLog(severity.WARNING, 1 /* depth */))
		require.False(t, l.shouldLog(severity.INFO, 1 /* depth */))
		require.True(t, l.shouldLog(severity.ERROR, 1 /* depth */))
	}

	{
		// When verbosity is nonzero, l.V() should reflect this.
		//
		// Also, we log at all severities when verbosity is
		// not zero.
		l := getGRPCLogger(severity.ERROR, "", "1000")
		require.Equal(t, l.grpcVerbosityLevel, 1000)
		require.True(t, l.V(1000))
		require.True(t, l.shouldLog(severity.INFO, 1 /* depth */))
		require.True(t, l.shouldLog(severity.WARNING, 1 /* depth */))
		require.True(t, l.shouldLog(severity.ERROR, 1 /* depth */))
		// NB: this could fail if we run with `--vmodule=*=<maxint32>`
		// but the hope is that we don't do that.
		require.False(t, l.V(math.MaxInt32))
	}
}
