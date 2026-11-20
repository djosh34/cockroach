// Copyright 2026 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package tracingpb

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
	"github.com/stretchr/testify/require"
)

// TestToJaegerJSON checks the fields that Jaeger reads from the trace.
func TestToJaegerJSON(t *testing.T) {
	start := timeutil.Unix(10, 0)
	rec := Recording{
		{TraceID: 7, SpanID: 1, Operation: "root", StartTime: start, Duration: time.Second},
		{TraceID: 7, SpanID: 2, ParentSpanID: 1, Operation: "child", StartTime: start, Duration: time.Millisecond, GoroutineID: 3},
	}
	out, err := rec.ToJaegerJSON("SELECT 1", "comment", "n1")
	require.NoError(t, err)

	var got struct {
		Data []struct {
			TraceID string `json:"traceID"`
			Spans   []struct {
				SpanID        string `json:"spanID"`
				OperationName string `json:"operationName"`
				StartTime     uint64 `json:"startTime"`
				ProcessID     string `json:"processID"`
				References    []struct {
					RefType string `json:"refType"`
					SpanID  string `json:"spanID"`
				} `json:"references"`
				Tags []struct {
					Key   string      `json:"key"`
					Type  string      `json:"type"`
					Value interface{} `json:"value"`
				} `json:"tags"`
			} `json:"spans"`
			Processes map[string]struct {
				ServiceName string `json:"serviceName"`
			} `json:"processes"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &got))
	require.Len(t, got.Data, 1)
	trace := got.Data[0]
	require.Equal(t, "7", trace.TraceID)
	require.Equal(t, "n1", trace.Processes["n1"].ServiceName)
	require.Len(t, trace.Spans, 2)
	require.Equal(t, "root", trace.Spans[0].OperationName)
	require.Equal(t, uint64(10_000_000), trace.Spans[0].StartTime)
	require.Equal(t, "n1", trace.Spans[0].ProcessID)
	child := trace.Spans[1]
	require.Equal(t, "2", child.SpanID)
	require.Len(t, child.References, 1)
	require.Equal(t, "CHILD_OF", child.References[0].RefType)
	require.Equal(t, "1", child.References[0].SpanID)
	require.Equal(t, "goroutine", child.Tags[0].Key)
	require.Equal(t, "int64", child.Tags[0].Type)
}
