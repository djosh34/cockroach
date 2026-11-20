// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package json

import (
	"github.com/cockroachdb/cockroach/pkg/util"
	"github.com/cockroachdb/errors"
)

// ParseOption is an option for JSON parsing.
type ParseOption interface {
	apply(cfg *parseConfig)
}

// WithUnorderedObjectKeys returns an option that leaves JSON
// object keys unordered.
func WithUnorderedObjectKeys() ParseOption {
	return funcOpt(func(cfg *parseConfig) {
		cfg.unordered = true
	})
}

// WithGoStandardParser returns an option that forces the use of Go parser
// (encoding/json).
func WithGoStandardParser() ParseOption {
	return funcOpt(func(cfg *parseConfig) {
		cfg.impl = useStdGoJSON
	})
}

// WithFastJSONParser returns an option that forces the use of fast json parser.
func WithFastJSONParser() ParseOption {
	return funcOpt(func(cfg *parseConfig) {
		cfg.impl = useFastJSONParser
	})
}

// parseJSONImplType is the implementation library
// used to parse JSON.
type parseJSONImplType int

const (
	// useStdGoJSON : encoding/json
	useStdGoJSON parseJSONImplType = iota
	// useFastJSONParser : uses fast JSON parser (built on top of pkg/json high performance library)
	useFastJSONParser

	// Note: Other libraries tested and rejected include:
	//  * json-iter: A fine library; however, fastJSONParser consistently
	//    performed better, with much better memory allocation behavior.
	//  * go-json: universally slower, worse (allocation) than either
	//    standard or json-iter for larger inputs.
	//  * simdjson-go: too restrictive; doesn't work on all platforms,
	//    does not parse raw json literals (true/false, etc); and is not
	//    a drop in replacement.
	//  * ffjson, easyjson: not appropriate since these library use code
	//    generation, and cannot parse arbitrary JSON.
)

// default configuration for parsing JSON.
var parseJSONDefaultConfig = func() (cfg parseConfig) {
	cfg.impl = util.ConstantWithMetamorphicTestChoice(
		"parse-json-impl", useFastJSONParser, useStdGoJSON,
	).(parseJSONImplType)
	return cfg
}()

type parseConfig struct {
	unordered bool
	impl      parseJSONImplType
}

func (c parseConfig) parseJSON(s string) (JSON, error) {
	switch c.impl {
	case useStdGoJSON:
		return parseJSONGoStd(s, c)
	case useFastJSONParser:
		return parseUsingFastParser(s, c)
	default:
		return nil, errors.AssertionFailedf("invalid ParseJSON implementation %v", c.impl)
	}
}

type funcOpt func(cfg *parseConfig)

func (f funcOpt) apply(cfg *parseConfig) {
	f(cfg)
}
