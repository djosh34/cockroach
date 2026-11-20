// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package tree

// Unlisten represents a UNLISTEN statement.
type Unlisten struct {
	ChannelName *UnresolvedObjectName
	Star        bool
}

var _ Statement = &Unlisten{}

// Format implements the NodeFormatter interface.
func (node *Unlisten) Format(ctx *FmtCtx) {
	ctx.WriteString("UNLISTEN ")
	if node.Star {
		ctx.WriteString("* ")
	} else if node.ChannelName != nil {
		ctx.FormatNode(node.ChannelName)
	}
}

// String implements the Statement interface.
func (node *Unlisten) String() string {
	return AsString(node)
}
