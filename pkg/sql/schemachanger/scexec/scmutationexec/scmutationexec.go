// Copyright 2020 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package scmutationexec

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/sql/schemachanger/scop"
	"github.com/cockroachdb/errors"
)

// NewImmediateVisitor creates a new scop.ImmediateMutationVisitor.
func NewImmediateVisitor(
	s ImmediateMutationStateUpdater, clock Clock, dr DescriptorReader,
) scop.ImmediateMutationVisitor {
	return &immediateVisitor{
		ImmediateMutationStateUpdater: s,
		clock:                         clock,
		descriptorReader:              dr,
	}
}

type immediateVisitor struct {
	ImmediateMutationStateUpdater
	clock            Clock
	descriptorReader DescriptorReader
}

var _ scop.ImmediateMutationVisitor = (*immediateVisitor)(nil)

// NewDeferredVisitor creates a new scop.DeferredMutationVisitor.
func NewDeferredVisitor(s DeferredMutationStateUpdater) scop.DeferredMutationVisitor {
	return &deferredVisitor{
		DeferredMutationStateUpdater: s,
	}
}

var _ scop.DeferredMutationVisitor = (*deferredVisitor)(nil)

type deferredVisitor struct {
	DeferredMutationStateUpdater
}

func (i *immediateVisitor) NotImplemented(_ context.Context, _ scop.NotImplemented) error {
	return errors.AssertionFailedf("not implemented operation was hit unexpectedly.")
}

func (i *immediateVisitor) NotImplementedForPublicObjects(
	ctx context.Context, op scop.NotImplementedForPublicObjects,
) error {
	desc := i.MaybeGetCheckedOutDescriptor(op.DescID)
	if desc == nil || desc.Dropped() {
		return nil
	}
	return errors.AssertionFailedf("not implemented operation was hit "+
		"unexpectedly, no dropped descriptor was found. %v", op.ElementType)
}
