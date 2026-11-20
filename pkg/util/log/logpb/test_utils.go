// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package logpb

import (
	"github.com/cockroachdb/cockroach/pkg/util/jsonbytes"
	"github.com/cockroachdb/redact"
)

// TestingStructuredLogEvent is an implementation of EventPayload for use in
// tests, in order to avoid importing the eventpb package.
type TestingStructuredLogEvent struct {
	CommonEventDetails
	Channel
	Event string
}

var _ EventPayload = (*TestingStructuredLogEvent)(nil)

// CommonDetails is part of the EventPayload interface.
func (f TestingStructuredLogEvent) CommonDetails() *CommonEventDetails {
	return &f.CommonEventDetails
}

// LoggingChannel is part of the EventPayload interface.
func (f TestingStructuredLogEvent) LoggingChannel() Channel {
	return f.Channel
}

// AppendJSONFields is part of the EventPayload interface.
func (f TestingStructuredLogEvent) AppendJSONFields(
	printComma bool, b redact.RedactableBytes,
) (bool, redact.RedactableBytes) {
	printComma, b = f.CommonEventDetails.AppendJSONFields(printComma, b)
	if f.Event != "" {
		if printComma {
			b = append(b, ',')
		}
		printComma = true
		b = append(b, "\"Event\":\""...)
		b = append(b, redact.StartMarker()...)
		b = redact.RedactableBytes(jsonbytes.EncodeString([]byte(b), string(redact.EscapeMarkers([]byte(f.Event)))))
		b = append(b, redact.EndMarker()...)
		b = append(b, '"')
	}
	return printComma, b
}
