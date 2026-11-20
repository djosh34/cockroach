// Copyright 2021 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package roachpb

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSpanConfigConformanceReport_IsEmpty(t *testing.T) {
	var r SpanConfigConformanceReport
	require.True(t, r.IsEmpty())
	r.ViolatingConstraints = make([]ConformanceReportedRange, 0)
	require.True(t, r.IsEmpty())
	r.ViolatingConstraints = append(r.ViolatingConstraints, ConformanceReportedRange{})
	require.False(t, r.IsEmpty())

	require.Equal(
		t, numSpanConfigConformanceReportSlices,
		reflect.TypeOf(&r).Elem().NumField(),
	)
}
