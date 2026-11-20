// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

// Package clienturl provides glue between:
//
//   - security/clientconnurl, which is able to translate configuration
//     parameters for SQL clients and compute security parameters,
//
// - cli/clisqlcfg, which serves as input for a SQL shell.
//
// It also provides the URL parser which is able to interleave
// itself in-between discrete flags.
//
// This logic is placed in a package separate from
// security/clientsecopts to avoid a dependency from
// security/clientsecopts into spf13/cobra and spf13/pflag.
package clienturl
