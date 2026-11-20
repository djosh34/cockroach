// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package sessiondatapb

import (
	"testing"

	"github.com/cockroachdb/cockroach/pkg/sql/protoreflect"
	"github.com/stretchr/testify/require"
)

func TestSessionDataJsonCompat(t *testing.T) {
	expectedSessionData := SessionData{
		VectorizeMode: VectorizeOn,
	}
	json, err := protoreflect.MessageToJSON(&expectedSessionData, protoreflect.FmtFlags{})
	require.NoError(t, err)
	actualSessionData := SessionData{}
	_, err = protoreflect.JSONBMarshalToMessage(json, &actualSessionData)
	require.NoError(t, err)
	require.Equal(t, expectedSessionData, actualSessionData)
}
