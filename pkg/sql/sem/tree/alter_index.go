// Copyright 2017 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

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
