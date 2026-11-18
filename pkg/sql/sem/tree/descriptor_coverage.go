// Copyright 2023 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package tree

// DescriptorCoverage specifies which descriptors an ingestion writes.
type DescriptorCoverage int32

const (
	// RequestedDescriptors means that only the requested tables and databases
	// are ingested, not all of the cluster data.
	RequestedDescriptors DescriptorCoverage = iota

	// AllDescriptors means that all of the relevant cluster data is ingested.
	AllDescriptors
)
