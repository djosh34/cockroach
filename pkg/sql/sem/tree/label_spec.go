// Copyright 2020 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package tree

// LabelSpec describes the labeling specification for an object.
type LabelSpec struct {
	IfNotExists bool
	Label       Expr
}

// Format implements the NodeFormatter interface.
func (l *LabelSpec) Format(ctx *FmtCtx) {
	if l.IfNotExists {
		ctx.WriteString(" IF NOT EXISTS")
	}
	if l.Label != nil {
		ctx.WriteString(" ")
		ctx.FormatNode(l.Label)
	}
}

var _ NodeFormatter = &LabelSpec{}
