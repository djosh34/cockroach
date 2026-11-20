// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package tsearch

import "strings"

// ValidConfig returns an error if the input string is not a supported and valid
// text search config.
func ValidConfig(input string) error {
	input = GetConfigKey(input)
	_, err := getStemmer(input)
	return err
}

// GetConfigKey returns a config that can be used as a key to look up stemmers
// and stopwords from an input config value. This is simulating the more
// advanced customizable dictionaries and configs that Postgres has, which
// allows user-defined text search configurations: because of this, configs can
// have schema prefixes. Because we don't (yet?) allow this, we just have to
// trim off any `pg_catalog.` prefix if it exists.
func GetConfigKey(config string) string {
	return strings.TrimPrefix(config, "pg_catalog.")
}
