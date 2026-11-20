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

	"github.com/cockroachdb/cockroach/pkg/sql/pgrepl/lsn"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
)

type StartReplication struct {
	Slot      tree.Name
	Temporary bool
	Kind      SlotKind
	LSN       lsn.LSN
	Options   Options
	Timeline  *tree.NumVal
}

var _ tree.Statement = (*StartReplication)(nil)

func (srs *StartReplication) String() string {
	return tree.AsString(srs)
}

func (srs *StartReplication) Format(ctx *tree.FmtCtx) {
	ctx.WriteString("START_REPLICATION")
	if srs.Slot != "" {
		ctx.WriteString(" SLOT ")
		ctx.FormatNode(&srs.Slot)
	}
	switch srs.Kind {
	case PhysicalReplication:
		ctx.WriteString(" PHYSICAL ")
	case LogicalReplication:
		ctx.WriteString(" LOGICAL ")
	default:
		panic(fmt.Sprintf("unknown replication slot kind: %d", srs.Kind))
	}
	ctx.WriteString(srs.LSN.String())
	if srs.Timeline != nil {
		ctx.WriteString(" TIMELINE ")
		ctx.FormatNode(srs.Timeline)
	}
	if len(srs.Options) > 0 {
		ctx.WriteString(" (")
		ctx.FormatNode(srs.Options)
		ctx.WriteString(")")
	}
}

func (srs *StartReplication) StatementReturnType() tree.StatementReturnType {
	return tree.Replication
}

func (srs *StartReplication) StatementType() tree.StatementType {
	return tree.TypeDDL
}

func (srs *StartReplication) StatementTag() string {
	return "START_REPLICATION"
}

func (srs *StartReplication) replicationStatement() {
}

var _ tree.Statement = (*StartReplication)(nil)
