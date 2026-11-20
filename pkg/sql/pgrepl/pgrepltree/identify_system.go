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

type IdentifySystem struct {
}

var _ tree.Statement = (*IdentifySystem)(nil)

func (i *IdentifySystem) String() string {
	return tree.AsString(i)
}

func (i *IdentifySystem) Format(ctx *tree.FmtCtx) {
	ctx.WriteString("IDENTIFY_SYSTEM")
}

func (i *IdentifySystem) StatementReturnType() tree.StatementReturnType {
	return tree.Rows
}

func (i *IdentifySystem) StatementType() tree.StatementType {
	return tree.TypeDML
}

func (i *IdentifySystem) StatementTag() string {
	return "IDENTIFY_SYSTEM"
}

func (i *IdentifySystem) replicationStatement() {
}

var _ tree.Statement = (*IdentifySystem)(nil)
