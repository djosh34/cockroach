// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package loopvarcapture

import (
	"fmt"
	"go/ast"
)

// Loop abstracts away the type of loop (`for` loop with index
// variable vs `range` loops)
type Loop struct {
	Vars []*ast.Ident
	Body *ast.BlockStmt
}

// NewLoop creates a new Loop struct according to the node passed. If
// the node does not represent either a `for` loop or a `range` loop,
// this function will panic.
func NewLoop(n ast.Node) *Loop {
	switch node := n.(type) {
	case *ast.ForStmt:
		return newForLoop(node)
	case *ast.RangeStmt:
		return newRange(node)
	default:
		panic(fmt.Errorf("unexpected loop node: %#v", n))
	}
}

// IsEmpty returns whether the loop is empty for the purposes of this
// linter; in other words, whether there no loop variables, or whether
// the loop has zero statements.
func (l *Loop) IsEmpty() bool {
	return len(l.Vars) == 0 || len(l.Body.List) == 0
}

func newForLoop(stmt *ast.ForStmt) *Loop {
	loop := Loop{Body: stmt.Body}

	switch post := stmt.Post.(type) {
	case *ast.AssignStmt:
		for _, lhs := range post.Lhs {
			loop.addVar(lhs)
		}

	case *ast.IncDecStmt:
		loop.addVar(post.X)
	}

	return &loop
}

func newRange(stmt *ast.RangeStmt) *Loop {
	loop := Loop{Body: stmt.Body}
	loop.addVar(stmt.Key)
	loop.addVar(stmt.Value)

	return &loop
}

func (l *Loop) addVar(e ast.Expr) {
	if ident, ok := e.(*ast.Ident); ok && ident.Obj != nil {
		l.Vars = append(l.Vars, ident)
	}
}
