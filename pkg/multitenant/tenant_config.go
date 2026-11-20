// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package multitenant

import (
	"time"

	"github.com/cockroachdb/cockroach/pkg/settings"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/catconstants"
)

// DefaultClusterSelectSettingName is the name of the setting that
// configures the default tenant to use when a client does not specify
// a specific tenant.
const DefaultClusterSelectSettingName = "server.controller.default_target_cluster"

// DefaultTenantSelect determines which tenant serves requests from
// clients that do not specify explicitly the tenant they want to use.
var DefaultTenantSelect = settings.RegisterStringSetting(
	settings.SystemOnly,
	"server.controller.default_tenant",
	"name of the virtual cluster to use when SQL or HTTP clients don't specify a target cluster",
	catconstants.SystemTenantName,
	settings.WithName(DefaultClusterSelectSettingName),
)

// VerifyTenantService determines whether there should be an advisory
// interlock between changes to the tenant service and changes to the
// above cluster setting.
var VerifyTenantService = settings.RegisterBoolSetting(
	settings.SystemOnly,
	"server.controller.default_tenant.check_service.enabled",
	"verify that the service mode is coherently set with the value of "+DefaultClusterSelectSettingName,
	true,
	settings.WithName(DefaultClusterSelectSettingName+".check_service.enabled"),
)

// WaitForClusterStartTimeout is the amount of time the tenant
// controller will wait for the default virtual cluster to have an
// active SQL server.
var WaitForClusterStartTimeout = settings.RegisterDurationSetting(
	settings.SystemOnly,
	"server.controller.mux_virtual_cluster_wait.timeout",
	"amount of time to wait for a default virtual cluster to become available when serving SQL connections (0 to disable)",
	10*time.Second,
)
