// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package keyvissettings

import (
	"time"

	"github.com/cockroachdb/cockroach/pkg/settings"
)

// Enabled enables the key visualizer job.
var Enabled = settings.RegisterBoolSetting(
	settings.SystemVisible,
	"keyvisualizer.enabled",
	"enable the use of the key visualizer", false)

// SampleInterval defines the sample period for key visualizer samples.
var SampleInterval = settings.RegisterDurationSetting(
	settings.SystemVisible,
	"keyvisualizer.sample_interval",
	"the frequency at which the key visualizer samples are collected",
	5*time.Minute,
	settings.DurationWithMinimum(1*time.Minute),
)

// MaxBuckets defines the maximum buckets allowed in a sample.
// Allowing more buckets will lead to a higher sampling resolution,
// at the cost of increased storage and network requirements.
var MaxBuckets = settings.RegisterIntSetting(
	settings.SystemOnly,
	"keyvisualizer.max_buckets",
	"the maximum number of buckets in a sample",
	256,
	settings.IntInRange(1, 1024),
)
