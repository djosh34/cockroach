// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package kvflowcontrol

import (
	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/kvflowcontrol/kvflowcontrolpb"
)

// TestingKnobs provide fine-grained control over the various kvflowcontrol
// components for testing.
type TestingKnobs struct {
	// UntrackTokensInterceptor is invoked whenever tokens are untracked, along
	// with their corresponding log positions.
	UntrackTokensInterceptor func(Tokens, kvflowcontrolpb.RaftLogPosition)
	// OverrideTokenDeduction is used to override how many tokens are deducted
	// post-evaluation.
	OverrideTokenDeduction func() Tokens
	// MaintainStreamsForBehindFollowers is used in tests to maintain
	// replication streams for behind followers.
	MaintainStreamsForBehindFollowers func() bool
	// MaintainStreamsForInactiveFollowers is used in tests to maintain
	// replication streams for inactive followers.
	MaintainStreamsForInactiveFollowers func() bool
	// MaintainStreamsForBrokenRaftTransport is used in tests to maintain
	// replication streams for followers we're no longer connected to via the
	// RaftTransport.
	MaintainStreamsForBrokenRaftTransport func() bool
	// UseOnlyForScratchRanges enables the use of kvflowcontrol
	// only for scratch ranges.
	UseOnlyForScratchRanges bool
}

// ModuleTestingKnobs is part of the base.ModuleTestingKnobs interface.
func (t *TestingKnobs) ModuleTestingKnobs() {}

var _ base.ModuleTestingKnobs = (*TestingKnobs)(nil)
