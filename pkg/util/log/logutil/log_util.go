// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package logutil

import (
	"context"
	"time"

	"github.com/cockroachdb/cockroach/pkg/jobs"
	"github.com/cockroachdb/cockroach/pkg/jobs/jobspb"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/log/eventpb"
	"github.com/cockroachdb/cockroach/pkg/util/log/logpb"
	"github.com/cockroachdb/cockroach/pkg/util/stop"
	"github.com/cockroachdb/redact"
)

// LogJobCompletion publishes an eventpb.RecoveryEvent about a successful or
// failed import, backup or restore job.
func LogJobCompletion(
	ctx context.Context,
	eventType eventpb.RecoveryEventType,
	jobID jobspb.JobID,
	success bool,
	jobErr error,
	numRows int64,
) {
	var redactedErr redact.RedactableString
	if jobErr != nil {
		redactedErr = redact.Sprint(jobErr)
	}
	status := jobs.StatusSucceeded
	if !success {
		if jobs.HasErrJobCanceled(jobErr) {
			status = jobs.StatusCanceled
		} else {
			status = jobs.StatusFailed
		}
	}

	event := &eventpb.RecoveryEvent{
		RecoveryType: eventType,
		JobID:        uint64(jobID),
		ResultStatus: string(status),
		ErrorText:    redactedErr,
		NumRows:      numRows,
	}

	log.StructuredEvent(ctx, event)
}

// LogEventsWithDelay logs an eventpb.EventPayload at provided
// delay duration to avoid exceeding the 10 log-line per second limit per node
// on the telemetry logging pipeline.
func LogEventsWithDelay(
	ctx context.Context, events []logpb.EventPayload, stopper *stop.Stopper, delay time.Duration,
) {
	// Log the first event immediately.
	timer := time.NewTimer(0 * time.Second)
	defer timer.Stop()
	for len(events) > 0 {
		select {
		case <-stopper.ShouldQuiesce():
			return
		case <-timer.C:
			event := events[0]
			log.StructuredEvent(ctx, event)
			events = events[1:]
			// Apply a delay to subsequent events.
			timer.Reset(delay)
		}
	}
}
