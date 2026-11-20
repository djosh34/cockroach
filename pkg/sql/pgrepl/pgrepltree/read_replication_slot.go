// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package pgrepltree

import "github.com/cockroachdb/cockroach/pkg/sql/sem/tree"

type ReadReplicationSlot struct {
	Slot tree.Name
}

var _ tree.Statement = (*ReadReplicationSlot)(nil)

func (rrs *ReadReplicationSlot) String() string {
	return tree.AsString(rrs)
}

func (rrs *ReadReplicationSlot) Format(ctx *tree.FmtCtx) {
	ctx.WriteString("READ_REPLICATION_SLOT ")
	ctx.FormatNode(&rrs.Slot)
}

func (rrs *ReadReplicationSlot) StatementReturnType() tree.StatementReturnType {
	return tree.Replication
}

func (rrs *ReadReplicationSlot) StatementType() tree.StatementType {
	return tree.TypeDDL
}

func (rrs *ReadReplicationSlot) StatementTag() string {
	return "READ_REPLICATION_SLOT"
}

func (rrs *ReadReplicationSlot) replicationStatement() {
}

var _ tree.Statement = (*ReadReplicationSlot)(nil)
