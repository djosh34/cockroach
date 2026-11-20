// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package grpcutil

import "github.com/cockroachdb/redact"

// gRPC logging is a real redirect maze but luckily it doesn't matter to get
// this right since it actually uses the DepthLoggerV2 interface. The methods
// here are provided as a fallback implementation in case there are any stray
// callsites in gRPC using the old interface.

func (l *grpcLogger) Info(args ...interface{}) {
	l.InfoDepth(0, args...)
}

func (l *grpcLogger) Infoln(args ...interface{}) {
	l.InfoDepth(0, args...)
}

func (l *grpcLogger) Infof(format string, args ...interface{}) {
	l.InfoDepth(0, redact.Sprintf(format, args...))
}

func (l *grpcLogger) Warning(args ...interface{}) {
	l.WarningDepth(0, args...)
}

func (l *grpcLogger) Warningln(args ...interface{}) {
	l.WarningDepth(0, args...)
}

func (l *grpcLogger) Warningf(format string, args ...interface{}) {
	l.WarningDepth(0, args...)
}

func (l *grpcLogger) Error(args ...interface{}) {
	l.ErrorDepth(0, args...)
}

func (l *grpcLogger) Errorln(args ...interface{}) {
	l.ErrorDepth(0, args...)
}

func (l *grpcLogger) Errorf(format string, args ...interface{}) {
	l.ErrorDepth(0, redact.Sprintf(format, args...))
}

func (l *grpcLogger) Fatal(args ...interface{}) {
	l.FatalDepth(0, args...)
}

func (l *grpcLogger) Fatalln(args ...interface{}) {
	l.FatalDepth(0, args...)
}

func (l *grpcLogger) Fatalf(format string, args ...interface{}) {
	l.FatalDepth(0, redact.Sprintf(format, args...))
}

func (l *grpcLogger) V(i int) bool {
	// NB: V isn't used by gRPC despite being in the interface.
	return l.vDepth(i, 0)
}
