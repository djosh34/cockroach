// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package spanconfigbounds

import (
	"fmt"

	"github.com/cockroachdb/cockroach/pkg/config"
	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/redact"
)

// Field corresponds to a field in the SpanConfig.
type Field interface {
	fmt.Stringer
	redact.SafeFormatter

	// FieldValue provides access to the field in a SpanConfig as
	// a value which can be displayed.
	FieldValue(c *roachpb.SpanConfig) Value

	// FieldBound provides access to the corresponding spanConfigBound implied by
	// a SpanConfigBounds as a value which can be displayed.
	FieldBound(b *Bounds) ValueBounds
}

// field is a thin interface used to reduce some ceremony around extracting
// the typed value for a field.
type field[T any] interface {
	Field
	fieldValue(c *roachpb.SpanConfig) *T
}

// Note that the order of fields in this slice matters. When clamping, fields
// will be clamped in this order. Order matters for fields like constraints and
// voterConstraints which consult other fields to determine the appropriate
// value.
var fields = [config.NumFields]Field{
	rangeMinBytes,
	rangeMaxBytes,
	globalReads,
	numVoters,
	numReplicas,
	gcTTLSeconds,
	constraints,
	voterConstraints,
	leasePreferences,
}

const (
	rangeMaxBytes    = int64Field(config.RangeMaxBytes)
	rangeMinBytes    = int64Field(config.RangeMinBytes)
	globalReads      = boolField(config.GlobalReads)
	numReplicas      = int32Field(config.NumReplicas)
	numVoters        = int32Field(config.NumVoters)
	gcTTLSeconds     = int32Field(config.GCTTL)
	constraints      = constraintsConjunctionField(config.Constraints)
	voterConstraints = constraintsConjunctionField(config.VoterConstraints)
	leasePreferences = leasePreferencesField(config.LeasePreferences)
)
