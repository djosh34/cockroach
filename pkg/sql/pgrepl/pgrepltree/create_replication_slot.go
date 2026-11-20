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

import (
	"fmt"

	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
)

type CreateReplicationSlot struct {
	Slot      tree.Name
	Temporary bool
	Kind      SlotKind
	Plugin    tree.Name
	Options   Options
}

var _ tree.Statement = (*CreateReplicationSlot)(nil)

func (crs *CreateReplicationSlot) String() string {
	return tree.AsString(crs)
}

func (crs *CreateReplicationSlot) Format(ctx *tree.FmtCtx) {
	ctx.WriteString("CREATE_REPLICATION_SLOT ")
	ctx.FormatNode(&crs.Slot)
	if crs.Temporary {
		ctx.WriteString(" TEMPORARY")
	}
	switch crs.Kind {
	case PhysicalReplication:
		ctx.WriteString(" PHYSICAL")
	case LogicalReplication:
		ctx.WriteString(" LOGICAL ")
		ctx.FormatNode(&crs.Plugin)
	default:
		panic(fmt.Sprintf("unknown replication slot kind: %d", crs.Kind))
	}
	if len(crs.Options) > 0 {
		ctx.WriteString(" (")
		ctx.FormatNode(crs.Options)
		ctx.WriteString(")")
	}
}

func (crs *CreateReplicationSlot) StatementReturnType() tree.StatementReturnType {
	return tree.Replication
}

func (crs *CreateReplicationSlot) StatementType() tree.StatementType {
	return tree.TypeDDL
}

func (crs *CreateReplicationSlot) StatementTag() string {
	return "CREATE_REPLICATION_SLOT"
}

func (crs *CreateReplicationSlot) replicationStatement() {
}

var _ tree.Statement = (*CreateReplicationSlot)(nil)
