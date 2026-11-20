// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

//go:build linux
// +build linux

package status

func getDefaultIgnoredDevices() string {
	// Excludes disks that have likely been counted elsewhere already, eg.
	// sda1 gets excluded because sda would count it instead, and nvme1n1p1 is
	// excluded as nvme1n1 is counted.
	//
	// This default regex is taken from Prometheus:
	// https://github.com/prometheus/node_exporter/blob/690efa61e86acefdf05bb4334a3d68128ded49c9/collector/diskstats_linux.go#L39
	return "^(ram|loop|fd|(h|s|v|xv)d[a-z]|nvme\\d+n\\d+p)\\d+$"
}
