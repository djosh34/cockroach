// Copyright 2026 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package tracingpb

// The types below are the parts of Jaeger's JSON trace format that
// ToJaegerJSON writes. The format is described here:
// https://github.com/jaegertracing/jaeger-ui/issues/381#issuecomment-494150826

// JaegerTrace is one trace.
type JaegerTrace struct {
	TraceID   string                   `json:"traceID"`
	Spans     []JaegerSpan             `json:"spans"`
	Processes map[string]JaegerProcess `json:"processes"`
	Warnings  []string                 `json:"warnings"`
}

// JaegerSpan is one span of a trace.
type JaegerSpan struct {
	TraceID       string            `json:"traceID"`
	SpanID        string            `json:"spanID"`
	OperationName string            `json:"operationName"`
	References    []JaegerReference `json:"references"`
	// StartTime is in microseconds since the Unix epoch.
	StartTime uint64 `json:"startTime"`
	// Duration is in microseconds.
	Duration  uint64           `json:"duration"`
	Tags      []JaegerKeyValue `json:"tags"`
	Logs      []JaegerLog      `json:"logs"`
	ProcessID string           `json:"processID,omitempty"`
	Warnings  []string         `json:"warnings"`
}

// JaegerReference links a span to its parent.
type JaegerReference struct {
	RefType string `json:"refType"`
	TraceID string `json:"traceID"`
	SpanID  string `json:"spanID"`
}

// JaegerProcess is the process, here the node, that produced spans.
type JaegerProcess struct {
	ServiceName string           `json:"serviceName"`
	Tags        []JaegerKeyValue `json:"tags"`
}

// JaegerLog is an event of a span.
type JaegerLog struct {
	// Timestamp is in microseconds since the Unix epoch.
	Timestamp uint64           `json:"timestamp"`
	Fields    []JaegerKeyValue `json:"fields"`
}

// JaegerKeyValue is a tag or a log field.
type JaegerKeyValue struct {
	Key   string      `json:"key"`
	Type  string      `json:"type,omitempty"`
	Value interface{} `json:"value"`
}
