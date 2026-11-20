// Copyright 2021 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package tracing

import (
	"github.com/cockroachdb/cockroach/pkg/util/tracing/tracingpb"
	"github.com/cockroachdb/redact"
	"github.com/gogo/protobuf/jsonpb"
)

// TraceToJSON returns the string representation of the trace in JSON format.
//
// TraceToJSON assumes that the first span in the recording contains all the
// other spans.
func TraceToJSON(trace tracingpb.Recording) (string, error) {
	root := normalizeSpan(trace[0], trace)
	marshaller := jsonpb.Marshaler{
		Indent: "\t",
	}
	str, err := marshaller.MarshalToString(&root)
	if err != nil {
		return "", err
	}
	return str, nil
}

func normalizeSpan(s tracingpb.RecordedSpan, trace tracingpb.Recording) tracingpb.NormalizedSpan {
	var n tracingpb.NormalizedSpan
	n.Operation = s.Operation
	n.StartTime = s.StartTime
	n.Duration = s.Duration
	n.TagGroups = s.TagGroups
	n.Logs = s.Logs
	n.StructuredRecords = s.StructuredRecords
	n.ChildrenMetadata = s.ChildrenMetadata

	for _, ss := range trace {
		if ss.ParentSpanID != s.SpanID {
			continue
		}
		n.Children = append(n.Children, normalizeSpan(ss, trace))
	}
	return n
}

// RedactAndTruncateError redacts the error and truncates the string
// representation of the error to a fixed length.
func RedactAndTruncateError(err error) string {
	maxErrLength := 250
	redactedErr := string(redact.Sprintf("%v", err))
	if len(redactedErr) < maxErrLength {
		maxErrLength = len(redactedErr)
	}
	return redactedErr[:maxErrLength]
}
