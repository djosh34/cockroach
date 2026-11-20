// Copyright 2017 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package tree

import "fmt"

// AlterIndexVisible represents a ALTER INDEX ... [VISIBLE | NOT VISIBLE] statement.
type AlterIndexVisible struct {
	Index        TableIndexName
	Invisibility IndexInvisibility
	IfExists     bool
}

var _ Statement = &AlterIndexVisible{}

// Format implements the NodeFormatter interface.
func (node *AlterIndexVisible) Format(ctx *FmtCtx) {
	ctx.WriteString("ALTER INDEX ")
	if node.IfExists {
		ctx.WriteString("IF EXISTS ")
	}
	ctx.FormatNode(&node.Index)
	switch {
	case node.Invisibility.FloatProvided:
		ctx.WriteString(" VISIBILITY " + fmt.Sprintf("%.2f", 1-node.Invisibility.Value))
	case node.Invisibility.Value == 1.0:
		ctx.WriteString(" NOT VISIBLE")
	default:
		ctx.WriteString(" VISIBLE")
	}
}
