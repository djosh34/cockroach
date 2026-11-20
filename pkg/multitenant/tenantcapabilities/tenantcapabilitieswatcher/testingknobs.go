// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package tenantcapabilitieswatcher

import (
	"github.com/cockroachdb/cockroach/pkg/base"
	"github.com/cockroachdb/cockroach/pkg/multitenant/tenantcapabilities"
)

// TestingKnobs contain testing helpers which are used by the Watcher.
type TestingKnobs struct {
	// WatcherRangeFeedKnobs control the lifecycle events for the underlying
	// rangefeed of the tenant capabilities Watcher.
	WatcherRangeFeedKnobs base.ModuleTestingKnobs

	// WatcherUpdatesInterceptor, if set, is called each time the Watcher
	// receives a set of updates.
	WatcherUpdatesInterceptor func(update tenantcapabilities.Update)
}

// ModuleTestingKnobs is part of the base.ModuleTestingKnobs interface.
func (t *TestingKnobs) ModuleTestingKnobs() {}

var _ base.ModuleTestingKnobs = (*TestingKnobs)(nil)
