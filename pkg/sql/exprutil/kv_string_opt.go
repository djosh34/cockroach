// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package exprutil

import (
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/errors"
)

// KVOptionValidationMap holds the valid entries and their policies for
// KVOptions.
type KVOptionValidationMap map[string]KVStringOptValidate

// KVStringOptValidate indicates the requested validation of a TypeAsStringOpts
// option.
type KVStringOptValidate string

// KVStringOptValidate values.
const (
	KVStringOptAny            KVStringOptValidate = `any`
	KVStringOptRequireNoValue KVStringOptValidate = `no-value`
	KVStringOptRequireValue   KVStringOptValidate = `value`
)

func (m KVOptionValidationMap) validate(opt tree.KVOption) error {
	k := string(opt.Key)
	validate, ok := m[k]
	if !ok {
		return errors.Errorf("invalid option %q", k)
	}

	if opt.Value == nil {
		if validate == KVStringOptRequireValue {
			return errors.Errorf("option %q requires a value", k)
		}
	} else if validate == KVStringOptRequireNoValue {
		return errors.Errorf("option %q does not take a value", k)
	}
	return nil
}
