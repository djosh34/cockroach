// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package importer

import (
	"github.com/cockroachdb/cockroach/pkg/settings"
	"github.com/cockroachdb/cockroach/pkg/util/mon"
)

// eventMemoryMultipier is the multiplier for the amount of memory needed to
// export a datum. Datums may be encoded into bytes and is buffered until the
// export file is written out. Since it's difficult to calculate the number of
// bytes that will be created, we use this multiplier for estimation.
var eventMemoryMultipier = settings.RegisterFloatSetting(
	settings.ApplicationLevel,
	"export.event_memory_multiplier",
	"the amount of memory required to export a datum is multiplied by this factor",
	3,
	settings.FloatWithMinimum(1),
)

// ExportTestingKnobs contains testing knobs for Export.
type ExportTestingKnobs struct {
	// MemoryMonitor is a test memory monitor to report allocations to.
	MemoryMonitor *mon.BytesMonitor
}

// ModuleTestingKnobs is part of the base.ModuleTestingKnobs interface.
func (*ExportTestingKnobs) ModuleTestingKnobs() {}
