// Copyright 2021 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package settingswatcher

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/settings"
)

// OverridesMonitor is an interface through which the settings watcher can
// receive setting overrides. Used for non-system tenants.
type OverridesMonitor interface {
	// WaitForStart waits until the overrides are ready for consumption.
	WaitForStart(ctx context.Context) error

	// Overrides retrieves the current set of setting overrides, as a
	// map from setting key to EncodedValue. Any settings that are
	// present must be set to the overridden value. It also returns a
	// channel that will be closed when the overrides are updated.
	Overrides() (map[settings.InternalKey]settings.EncodedValue, <-chan struct{})
}
