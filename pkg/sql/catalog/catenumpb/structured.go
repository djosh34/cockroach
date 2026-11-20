// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package catenumpb

// IndexDescriptorEncodingType is a custom type to represent different encoding types
// for secondary indexes.
type IndexDescriptorEncodingType uint32

const (
	// SecondaryIndexEncoding corresponds to the standard way of encoding secondary indexes
	// as described in docs/tech-notes/encoding.md. We allow the 0 value of this type
	// to have a value so that existing descriptors are encoding using this encoding.
	SecondaryIndexEncoding IndexDescriptorEncodingType = iota
	// PrimaryIndexEncoding corresponds to when a secondary index is encoded using the
	// primary index encoding as described in docs/tech-notes/encoding.md.
	PrimaryIndexEncoding
)

// Remove unused warning.
var _ = SecondaryIndexEncoding
