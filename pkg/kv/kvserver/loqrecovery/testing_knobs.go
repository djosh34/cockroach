// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package loqrecovery

import (
	"time"

	"github.com/cockroachdb/cockroach/pkg/server/serverpb"
)

type TestingKnobs struct {
	// MetadataScanTimeout controls how long loss of quorum recovery service will
	// wait for range metadata. Useful to speed up tests verifying collection
	// behaviors when meta ranges are unavailable.
	MetadataScanTimeout time.Duration

	// Replica filter for forwarded replica info when collecting fan-out data.
	// It can be called concurrently, so must be safe for concurrent use.
	ForwardReplicaFilter func(*serverpb.RecoveryCollectLocalReplicaInfoResponse) error
}

// ModuleTestingKnobs is part of the base.ModuleTestingKnobs interface.
func (*TestingKnobs) ModuleTestingKnobs() {}
