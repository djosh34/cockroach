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

type DropReplicationSlot struct {
	Slot tree.Name
	Wait bool
}

var _ tree.Statement = (*DropReplicationSlot)(nil)

func (drs *DropReplicationSlot) String() string {
	return tree.AsString(drs)
}

func (drs *DropReplicationSlot) Format(ctx *tree.FmtCtx) {
	ctx.WriteString("DROP_REPLICATION_SLOT ")
	ctx.FormatNode(&drs.Slot)
	if drs.Wait {
		ctx.WriteString(" WAIT")
	}
}

func (drs *DropReplicationSlot) StatementReturnType() tree.StatementReturnType {
	return tree.Replication
}

func (drs *DropReplicationSlot) StatementType() tree.StatementType {
	return tree.TypeDDL
}

func (drs *DropReplicationSlot) StatementTag() string {
	return "DROP_REPLICATION_SLOT"
}

func (drs *DropReplicationSlot) replicationStatement() {
}

var _ tree.Statement = (*DropReplicationSlot)(nil)
