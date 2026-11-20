// Copyright 2020 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

// Package upgrade captures the facilities needed to define and execute
// upgrades for a crdb cluster. These upgrades can be arbitrarily long
// running, are free to send out arbitrary requests cluster wide, change
// internal DB state, and much more. They're typically reserved for crdb
// internal operations and state. Each upgrade is idempotent in nature, is
// associated with a specific cluster version, and executed when the cluster
// version is made active on every node in the cluster.
//
// Examples of upgrades that apply would be upgrades to move all raft state
// from one storage engine to another, or purging all usage of the replicated
// truncated state in KV.
package upgrade

import (
	"fmt"

	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/upgrade/upgradebase"
)

// JobDeps are upgrade-specific dependencies used by the upgrade job to run
// upgrades.
type JobDeps interface {

	// GetUpgrade returns the upgrade associated with the cluster version
	// if one exists.
	GetUpgrade(key roachpb.Version) (upgradebase.Upgrade, bool)

	// SystemDeps returns a handle to upgrade dependencies on a system tenant.
	SystemDeps() SystemDeps
}

type upgrade struct {
	description string
	// v is the version that this upgrade is associated with. The upgrade runs
	// when the cluster's version is incremented to v or, for permanent upgrades
	// (see below) when the cluster is bootstrapped at v or above.
	v roachpb.Version
	// permanent is set for "permanent" upgrades - i.e. upgrades that are not
	// baked into the bootstrap image and need to be run on new clusters
	// regardless of the cluster's bootstrap version.
	permanent bool
	// v22_2StartupMigrationName, if set, is the name of the corresponding
	// startupmigration in 22.2. In 23.1, we've turned these startupmigrations
	// into permanent upgrades. We don't want to run the upgrade if the
	// startupmigration had run.
	v22_2StartupMigrationName string
}

// Version is part of the upgradebase.Upgrade interface.
func (m *upgrade) Version() roachpb.Version {
	return m.v
}

// Permanent is part of the upgradebase.Upgrade interface.
func (m *upgrade) Permanent() bool {
	return m.permanent
}

// Name is part of the upgradebase.Upgrade interface.
func (m *upgrade) Name() string {
	return fmt.Sprintf("Upgrade to %s: %q", m.v.String(), m.description)
}

// V22_2StartupMigrationName is part of the upgradebase.Upgrade interface.
func (m *upgrade) V22_2StartupMigrationName() string {
	if !m.permanent {
		panic("V22_2StartupMigrationName() called on non-permanent upgrade.")
	}
	return m.v22_2StartupMigrationName
}
