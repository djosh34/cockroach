// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package rangetestutils

import (
	"testing"

	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/testutils/serverutils"
)

// TestServerFactory creates test servers with an initial set of
// inspectable ranges.
type TestServerFactory interface {
	// MakeRangeTestServerArgs instantiates TestServerArgs suitable
	// to instantiate a range test server.
	MakeRangeTestServerArgs() base.TestServerArgs
	// PrepareRangeTestServer prepares a range test server for use.
	PrepareRangeTestServer(testServer interface{}) error
}

var srvFactoryImpl TestServerFactory

// InitTestServerFactory should be called once to provide the
// implementation of the service. It will be called from a xx_test
// package that can import the server package.
func InitRangeTestServerFactory(impl TestServerFactory) {
	srvFactoryImpl = impl
}

// StartServer starts a server with multiple stores, a short scan
// interval, wait for the scan to complete, and return the server. The
// caller is responsible for stopping the server.
func StartServer(t testing.TB) serverutils.TestServerInterface {
	params := srvFactoryImpl.MakeRangeTestServerArgs()
	s := serverutils.StartServerOnly(t, params)
	if err := srvFactoryImpl.PrepareRangeTestServer(s); err != nil {
		t.Fatal(err)
	}
	return s
}
