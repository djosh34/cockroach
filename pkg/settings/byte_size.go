// Copyright 2017 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package settings

import (
	"github.com/cockroachdb/cockroach/pkg/util/humanizeutil"
	"github.com/cockroachdb/errors"
)

// ByteSizeSetting is the interface of a setting variable that will be
// updated automatically when the corresponding cluster-wide setting
// of type "bytesize" is updated.
type ByteSizeSetting struct {
	IntSetting
}

var _ numericSetting = &ByteSizeSetting{}

// Typ returns the short (1 char) string denoting the type of setting.
func (*ByteSizeSetting) Typ() string {
	return "z"
}

func (b *ByteSizeSetting) String(sv *Values) string {
	return string(humanizeutil.IBytes(b.Get(sv)))
}

// DecodeToString decodes and renders an encoded value.
func (b *ByteSizeSetting) DecodeToString(encoded string) (string, error) {
	iv, err := b.DecodeValue(encoded)
	if err != nil {
		return "", err
	}
	return string(humanizeutil.IBytes(iv)), nil
}

// DefaultString returns the default value for the setting as a string.
func (b *ByteSizeSetting) DefaultString() (string, error) {
	return b.DecodeToString(b.EncodedDefault())
}

// RegisterByteSizeSetting defines a new setting with type bytesize and any
// supplied validation function(s). If no validation functions are given, then
// the non-negative int validation is performed.
func RegisterByteSizeSetting(
	class Class, key InternalKey, desc string, defaultValue int64, opts ...SettingOption,
) *ByteSizeSetting {
	validateFn := func(v int64) error {
		hasExplicitValidationFn := false
		for _, opt := range opts {
			switch {
			case opt.commonOpt != nil:
				continue
			case opt.validateInt64Fn != nil:
			default:
				panic(errors.AssertionFailedf("wrong validator type"))
			}
			hasExplicitValidationFn = true
			if err := opt.validateInt64Fn(v); err != nil {
				return errors.Wrapf(err, "invalid value for %s", key)
			}
		}
		if !hasExplicitValidationFn {
			// Default validation.
			return nonNegativeIntInternal(v)
		}
		return nil
	}

	if err := validateFn(defaultValue); err != nil {
		panic(errors.Wrap(err, "invalid default"))
	}
	setting := &ByteSizeSetting{IntSetting{
		defaultValue: defaultValue,
		validateFn:   validateFn,
	}}
	register(class, key, desc, setting)
	setting.apply(opts)
	return setting
}

// ByteSizeWithMinimum can be passed to RegisterByteSizeSetting.
func ByteSizeWithMinimum(minVal int64) SettingOption {
	return WithValidateInt(func(v int64) error {
		if v < minVal {
			return errors.Errorf("cannot be set to a value lower than %v",
				humanizeutil.IBytes(minVal))
		}
		return nil
	})
}
