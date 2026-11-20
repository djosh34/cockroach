// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package syntheticprivilege

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParsePrivilegePath(t *testing.T) {
	for _, tc := range []struct {
		regex        string
		expectedType Object
		error        string
	}{
		{
			regex: "/garbageasdf/",
			error: "no prefix match found for privilege path /garbageasdf/",
		},
		{
			regex:        "/global/",
			expectedType: GlobalPrivilegeObject,
		},
		{
			regex: "/global/unexpected",
			error: "/global/unexpected does not match regex pattern (/global/)$",
		},
	} {
		actualType, err := Parse(tc.regex)
		if tc.error != "" {
			if err == nil {
				t.Fatalf("expected error %s, no error was given", tc.error)
			}
			require.Equal(t, tc.error, err.Error())
		} else {
			require.NoError(t, err)
			require.Equal(t, reflect.TypeOf(actualType), reflect.TypeOf(tc.expectedType))
			require.Equal(t, actualType.GetPath(), tc.expectedType.GetPath())
		}
	}
}
