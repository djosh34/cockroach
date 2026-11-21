// Copyright 2016 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.
//
// This file provides generic interfaces that allow tests to set up test tenants
// without importing the server package (avoiding circular dependencies). This

package serverutils

import (
	"net/url"

	"github.com/cockroachdb/errors"
)

// PreventDisableSQLForTenantError is thrown by tests that attempt to set
// DisableSQLServer but run with cluster virtualization. These modes are
// incompatible since tenant operations require SQL during initialization.
func PreventDisableSQLForTenantError() error {
	return errors.New("programming error: DisableSQLServer is incompatible with cluster virtualization\n" +
		"Consider using TestServerArgs{DefaultTestTenant: base.TestIsSpecificToStorageLayerAndNeedsASystemTenant}")
}

type TestURL struct {
	*url.URL
}

func NewTestURL(path string) TestURL {
	u, err := url.Parse(path)
	if err != nil {
		panic(err)
	}
	return TestURL{u}
}

// WithPath is a helper that allows the user of the `TestURL` to easily append
// paths to use for testing. Any queries in the given path will be added to the
// returned URL. Please be aware that your path will automatically be escaped
// for you, but if it includes any invalid hex escapes (eg: `%s`) it will fail
// silently, and you'll get back a blank URL.
func (t *TestURL) WithPath(path string) *TestURL {
	parsedPath, err := url.Parse(path)
	if err != nil {
		panic(err)
	}
	// Append the path to the existing path (The paths used here do not contain any
	// query parameters). To prevent double escaping, we use the `JoinPath` method.
	t.URL = t.JoinPath(parsedPath.EscapedPath())
	// Append the query parameters to the existing query parameters.
	query := t.Query()
	for k, v := range parsedPath.Query() {
		query[k] = v
	}
	t.RawQuery = query.Encode()
	return t
}
