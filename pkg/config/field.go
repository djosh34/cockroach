// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

// Package config contains basic utilities and data definitions for zone
// configuration.
package config

import "github.com/cockroachdb/redact"

// Field corresponds to a field in either a SpanConfig or ZoneConfig.
type Field int

// SafeValue makes Field a SafeValue.
func (i Field) SafeValue() {}

var _ redact.SafeValue = Field(0)

//go:generate stringer --type=Field --linecomment

const (
	_                Field = iota
	RangeMinBytes          // range_min_bytes
	RangeMaxBytes          // range_max_bytes
	GlobalReads            // global_reads
	NumReplicas            // num_replicas
	NumVoters              // num_voters
	GCTTL                  // gc.ttlseconds
	Constraints            // constraints
	VoterConstraints       // voter_constraints
	LeasePreferences       // lease_preferences

	// NumFields is the number of fields in the config.
	NumFields int = iota - 1
)
