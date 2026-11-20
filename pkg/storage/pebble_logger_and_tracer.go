// Copyright 2024 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package storage

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/pebble"
)

type pebbleLogger struct {
	ctx   context.Context
	depth int
}

var _ pebble.LoggerAndTracer = pebbleLogger{}

func (l pebbleLogger) Infof(format string, args ...interface{}) {
	log.Storage.InfofDepth(l.ctx, l.depth, format, args...)
}

func (l pebbleLogger) Fatalf(format string, args ...interface{}) {
	log.Storage.FatalfDepth(l.ctx, l.depth, format, args...)
}

// pebble.LoggerAndTracer does not expose verbosity levels in its logging
// interface, and Pebble logs go to a separate STORAGE channel.
//
// The tracing part of the interface is meant for user-facing activities, so
// in addition to outputting the event when tracing is enabled, we also log.
// The eventAlsoLogVerbosityLevel of 2 is chosen semi-arbitrarily since this
// is the only verbosity level in this file.
const eventAlsoLogVerbosityLevel = 2

func (l pebbleLogger) Eventf(ctx context.Context, format string, args ...interface{}) {
	log.VEventf(ctx, eventAlsoLogVerbosityLevel, format, args...)
}

func (l pebbleLogger) IsTracingEnabled(ctx context.Context) bool {
	return log.HasSpanOrEvent(ctx) || log.ExpensiveLogEnabled(ctx, eventAlsoLogVerbosityLevel)
}
