// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package cli

import "os"

// getDefaultHost gets the default value for the host to connect to. pgx already
// has logic that would inspect PGHOST, but the problem is that if PGHOST not
// defined, pgx would default to using a unix socket. That is not desired, so
// here we make the CLI fallback to use "localhost" if PGHOST is not defined.
func getDefaultHost() string {
	if h := os.Getenv("PGHOST"); h == "" {
		return "localhost"
	}
	return ""
}
