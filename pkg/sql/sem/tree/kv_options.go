// Copyright 2016 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package tree

// KVOption is a key-value option.
type KVOption struct {
	Key   Name
	Value Expr
}

// KVOptions is a list of KVOptions.
type KVOptions []KVOption

// HasKey searches the set of options to discover if it has key.
func (o *KVOptions) HasKey(key Name) bool {
	for _, kv := range *o {
		if kv.Key == key {
			return true
		}
	}
	return false
}

// Format implements the NodeFormatter interface.
func (o *KVOptions) Format(ctx *FmtCtx) {
	o.formatEach(ctx, func(n *KVOption, ctx *FmtCtx) {
		ctx.FormatNode(n.Value)
	})
}

// formatEach is like Format but allows custom formatting of the value part.
func (o *KVOptions) formatEach(ctx *FmtCtx, formatValue func(*KVOption, *FmtCtx)) {
	for i := range *o {
		n := &(*o)[i]
		if i > 0 {
			ctx.WriteString(", ")
		}
		// KVOption Key values never contain PII and should be distinguished
		// for feature tracking purposes.
		ctx.WithFlags(ctx.flags&^FmtMarkRedactionNode, func() {
			ctx.FormatNode(&n.Key)
		})
		if n.Value != nil {
			ctx.WriteString(` = `)
			formatValue(n, ctx)
		}
	}
}

// StringOrPlaceholderOptList is a list of strings or placeholders.
type StringOrPlaceholderOptList []Expr

// Format implements the NodeFormatter interface.
func (node *StringOrPlaceholderOptList) Format(ctx *FmtCtx) {
	if len(*node) > 1 {
		ctx.WriteString("(")
	}
	ctx.FormatNode((*Exprs)(node))
	if len(*node) > 1 {
		ctx.WriteString(")")
	}
}
