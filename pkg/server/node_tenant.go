// Copyright 2021 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package server

import "github.com/cockroachdb/cockroach/pkg/util/tracing/tracingpb"

// redactRecording redacts the sensitive parts of log messages in the recording.
// It is used to record KV traces going to tenants other than the sytem tenant
// (the system tenant can receive unredacted recordings).
//
// Unstructured log messages get redacted and tags get stripped. Structured
// messages are left untouched (for better or worse).
// The unstructured log messages get completely dropped unless the
// `trace.redactable.enabled` cluster setting is set. This setting makes the log
// messages properly redactable (i.e. only the sensitive parts are wrapped in
// redaction markers) at some performance cost, whereas without it each log
// message is wholly wrapped in redaction markers.
//
// The recording is modified in place.
func redactRecording(rec tracingpb.Recording) {
	for i := range rec {
		sp := &rec[i]
		sp.TagGroups = nil
		for j := range sp.Logs {
			record := &sp.Logs[j]
			record.Message = record.Message.Redact()
		}
	}
}
