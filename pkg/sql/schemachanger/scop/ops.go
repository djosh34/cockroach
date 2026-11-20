// Copyright 2020 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

// Package scop describes ops within a schema change.
package scop

// Op represents an action to be taken on a single descriptor.
type Op interface {
	Type() Type
}

// Type represents the type of operation for an Op. Ops can be grouped into the
// same Stage only if they share a type.
type Type int

//go:generate stringer -type=Type

const (
	_ Type = iota
	// MutationType represents metadata changes.
	MutationType
	// BackfillType represents index backfills.
	BackfillType
	// ValidationType represents constraint and unique index validations
	// performed using internal queries.
	ValidationType

	maxType int = iota - 1
)

// IsValid is true if the Type has a valid value.
func (t Type) IsValid() bool {
	return t > 0 && t <= Type(maxType)
}

type baseOp struct{}
