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

type BaseBackup struct {
	Options Options
}

var _ tree.Statement = (*BaseBackup)(nil)

func (bb *BaseBackup) String() string {
	return tree.AsString(bb)
}

func (bb *BaseBackup) Format(ctx *tree.FmtCtx) {
	ctx.WriteString("BASE_BACKUP")
	if len(bb.Options) > 0 {
		ctx.WriteString(" (")
		ctx.FormatNode(bb.Options)
		ctx.WriteString(")")
	}
}

func (bb *BaseBackup) StatementReturnType() tree.StatementReturnType {
	return tree.Replication
}

func (bb *BaseBackup) StatementType() tree.StatementType {
	return tree.TypeDDL
}

func (bb *BaseBackup) StatementTag() string {
	return "BASE_BACKUP"
}

func (bb *BaseBackup) replicationStatement() {
}

var _ tree.Statement = (*BaseBackup)(nil)
