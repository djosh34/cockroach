// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package admission

import (
	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/util/admission/admissionpb"
)

// TestingKnobs provide fine-grained control over the various admission control
// components for testing.
//
// TODO(irfansharif): Consolidate the various other testing-knob like things (in
// admission.Options, for example) into this one struct.
type TestingKnobs struct {
	// AdmittedReplicatedWorkInterceptor is invoked whenever replicated work is
	// admitted.
	AdmittedReplicatedWorkInterceptor func(
		tenantID roachpb.TenantID,
		pri admissionpb.WorkPriority,
		rwi ReplicatedWorkInfo,
		originalTokens int64,
		createTime int64,
	)

	// DisableWorkQueueFastPath disables the fast-path in work queues.
	DisableWorkQueueFastPath bool

	// DisableWorkQueueGranting disables the work queue from granting admission
	// to waiting work.
	DisableWorkQueueGranting func() bool

	// AlwaysTryGrantWhenAdmitted causes the granter to unconditionally try
	// admitting another request when admitting one.
	AlwaysTryGrantWhenAdmitted bool
}

// ModuleTestingKnobs is part of the base.ModuleTestingKnobs interface.
func (t *TestingKnobs) ModuleTestingKnobs() {}

var _ base.ModuleTestingKnobs = (*TestingKnobs)(nil)
