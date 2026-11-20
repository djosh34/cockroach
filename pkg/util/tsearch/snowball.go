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

import (
	"github.com/blevesearch/snowballstem"
	"github.com/blevesearch/snowballstem/danish"
	"github.com/blevesearch/snowballstem/dutch"
	"github.com/blevesearch/snowballstem/english"
	"github.com/blevesearch/snowballstem/finnish"
	"github.com/blevesearch/snowballstem/french"
	"github.com/blevesearch/snowballstem/german"
	"github.com/blevesearch/snowballstem/hungarian"
	"github.com/blevesearch/snowballstem/italian"
	"github.com/blevesearch/snowballstem/norwegian"
	"github.com/blevesearch/snowballstem/portuguese"
	"github.com/blevesearch/snowballstem/russian"
	"github.com/blevesearch/snowballstem/spanish"
	"github.com/blevesearch/snowballstem/swedish"
	"github.com/blevesearch/snowballstem/turkish"
	"github.com/cockroachdb/cockroach/pkg/sql/pgwire/pgcode"
	"github.com/cockroachdb/cockroach/pkg/sql/pgwire/pgerror"
)

func getStemmer(config string) (func(env *snowballstem.Env) bool, error) {
	switch config {
	case "simple":
		return func(env *snowballstem.Env) bool {
			return true
		}, nil
	case "english":
		return english.Stem, nil
	case "danish":
		return danish.Stem, nil
	case "dutch":
		return dutch.Stem, nil
	case "finnish":
		return finnish.Stem, nil
	case "french":
		return french.Stem, nil
	case "german":
		return german.Stem, nil
	case "hungarian":
		return hungarian.Stem, nil
	case "italian":
		return italian.Stem, nil
	case "norwegian":
		return norwegian.Stem, nil
	case "portuguese":
		return portuguese.Stem, nil
	case "russian":
		return russian.Stem, nil
	case "spanish":
		return spanish.Stem, nil
	case "swedish":
		return swedish.Stem, nil
	case "turkish":
		return turkish.Stem, nil
	}
	return nil, pgerror.Newf(pgcode.UndefinedObject, "text search configuration %q does not exist", config)
}
