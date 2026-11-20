// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package tree

// Batch represents a BATCH clause.
type Batch struct {
	Params []BatchParam
}

var _ NodeFormatter = &Batch{}

type BatchParam interface {
	NodeFormatter
}

// SizeBatchParam represents a BATCH (SIZE size) parameter.
type SizeBatchParam struct {
	// Size is the expression specified by SIZE <size>.
	// It must be positive.
	Size Expr
}

// BatchParam represents a BATCH (param) parameter.
var _ BatchParam = &SizeBatchParam{}

// Format implements NodeFormatter.
func (p *SizeBatchParam) Format(ctx *FmtCtx) {
	ctx.WriteString("SIZE ")
	p.Size.Format(ctx)
}

// Format implements the NodeFormatter interface.
func (b *Batch) Format(ctx *FmtCtx) {
	if b == nil {
		return
	}
	ctx.WriteString("BATCH ")
	params := b.Params
	if len(params) > 0 {
		ctx.WriteString("(")
		for i, param := range params {
			if i > 0 {
				ctx.WriteString(",")
			}
			param.Format(ctx)
		}
		ctx.WriteString(") ")
	}
}
