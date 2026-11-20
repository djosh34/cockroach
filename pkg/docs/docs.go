// Copyright 2020 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package docs

// URLBase is the root URL of the CockroachDB v23.2 docs. patchroaddb is a
// fork of CockroachDB v23.2, so those docs describe it, except for the
// features patchroaddb removed.
const URLBase = "https://www.cockroachlabs.com/docs/v23.2"

// URLReleaseNotesBase is the root URL of the CockroachDB v23.2.0 release
// notes.
const URLReleaseNotesBase = "https://www.cockroachlabs.com/docs/releases/v23.2.0.html"

// URL generates the URL to pageName in the version of the docs associated
// with this binary.
func URL(pageName string) string { return URLBase + "/" + pageName }

// ReleaseNotesURL generates the URL to pageName in the .0 patch release notes
// docs associated with this binary.
func ReleaseNotesURL(pageName string) string { return URLReleaseNotesBase + pageName }
