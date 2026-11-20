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
	"bytes"
	"embed"
	"path"
	"strings"
)

//go:embed stopwords/*
var stopwordFS embed.FS

var stopwordsMap map[string]map[string]struct{}

func init() {
	stopwordsMap = make(map[string]map[string]struct{})
	dir, err := stopwordFS.ReadDir("stopwords")
	if err != nil {
		panic("error loading stopwords: " + err.Error())
	}
	for _, f := range dir {
		filename := f.Name()
		name := strings.TrimSuffix(filename, ".stop")
		// N.B. we use path.Join here instead of filepath.Join because go:embed
		// always uses forward slashes. https://github.com/golang/go/issues/45230
		contents, err := stopwordFS.ReadFile(path.Join("stopwords", filename))
		if err != nil {
			panic("error loading stopwords: " + err.Error())
		}
		wordList := bytes.Fields(contents)
		stopwordsMap[name] = make(map[string]struct{}, len(wordList))
		for _, word := range wordList {
			stopwordsMap[name][string(word)] = struct{}{}
		}
	}
	// The simple text search config has no stopwords.
	stopwordsMap["simple"] = nil
}
