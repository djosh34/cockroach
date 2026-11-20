// Copyright 2023 The Cockroach Authors.
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
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

var b = RegisterBoolSetting(SystemOnly, "b", "desc", true)

func TestIgnoreDefaults(t *testing.T) {
	ctx := context.Background()
	sv := &Values{}
	sv.Init(ctx, TestOpaque)

	ignoreAllUpdates = true
	defer func() { ignoreAllUpdates = false }()
	u := NewUpdater(sv)
	require.NoError(t, u.Set(ctx, b.InternalKey(), EncodedValue{Value: EncodeBool(false), Type: "b"}))
	require.Equal(t, true, b.Get(sv))

	ignoreAllUpdates = false
	u = NewUpdater(sv)
	require.NoError(t, u.Set(ctx, b.InternalKey(), EncodedValue{Value: EncodeBool(false), Type: "b"}))
	require.Equal(t, false, b.Get(sv))
}

var sensitiveSetting = RegisterStringSetting(
	ApplicationLevel,
	"my.sensitive.setting",
	"description",
	"",
	Sensitive,
)

func TestSensitiveSetting(t *testing.T) {
	ctx := context.Background()
	sv := &Values{}
	sv.Init(ctx, TestOpaque)

	require.True(t, sensitiveSetting.isSensitive())
	u := NewUpdater(sv)
	require.NoError(t, u.Set(ctx, sensitiveSetting.InternalKey(), EncodedValue{Value: "foo", Type: "s"}))

	for _, canViewSensitive := range []bool{true, false} {
		t.Run(fmt.Sprintf("%s=%v", "canViewSensitive", canViewSensitive), func(t *testing.T) {
			for _, redactionEnabled := range []bool{true, false} {
				t.Run(fmt.Sprintf("%s=%v", "redactionEnabled", redactionEnabled), func(t *testing.T) {
					redactSensitiveSettingsEnabled.Override(ctx, sv, redactionEnabled)
					underTest, ok := LookupForDisplayByKey(sensitiveSetting.InternalKey(), ForSystemTenant, canViewSensitive)
					require.True(t, ok)

					expectedValue := "foo"
					var expectedType Setting
					if canViewSensitive {
						expectedType = &StringSetting{}
					} else {
						expectedType = &MaskedSetting{}
						if redactionEnabled {
							expectedValue = "<redacted>"
						}
					}

					require.IsType(t, expectedType, underTest)
					actual := underTest.String(sv)
					require.Equal(t, expectedValue, actual)
				})
			}
		})
	}
}
