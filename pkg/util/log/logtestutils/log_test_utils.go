// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package logtestutils

import (
	"testing"

	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/log/logconfig"
	"github.com/cockroachdb/cockroach/pkg/util/log/logpb"
)

// InstallLogFileSink installs a file sink for logging tests.
func InstallLogFileSink(sc *log.TestLogScope, t *testing.T, channel logpb.Channel) func() {
	// Enable logging channels.
	log.TestingResetActive()
	cfg := logconfig.DefaultConfig()
	// Make a sink for just the session log.
	cfg.Sinks.FileGroups = make(map[string]*logconfig.FileSinkConfig)
	fileSinkConfig := logconfig.FileSinkConfig{Channels: logconfig.SelectChannels(channel)}
	switch channel {
	case logpb.Channel_TELEMETRY:
		cfg.Sinks.FileGroups["telemetry"] = &fileSinkConfig
	case logpb.Channel_SENSITIVE_ACCESS:
		cfg.Sinks.FileGroups["sql-audit"] = &fileSinkConfig
	case logpb.Channel_SQL_EXEC:
		cfg.Sinks.FileGroups["sql-exec"] = &fileSinkConfig
	default:
		panic("unrecognized logging channel")
	}
	dir := sc.GetDirectory()
	if err := cfg.Validate(&dir); err != nil {
		t.Fatal(err)
	}
	cleanup, err := log.ApplyConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	return cleanup
}
