// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package pgreplparser

import (
	"github.com/cockroachdb/cockroach/pkg/sql/parser/statements"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/errors"
)

func Parse(sql string) (statements.Statement[tree.Statement], error) {
	lexer := newLexer(sql)
	p := pgreplNewParser()
	if p.Parse(lexer) != 0 {
		if lexer.lastError == nil {
			return statements.Statement[tree.Statement]{}, errors.AssertionFailedf("expected lexer error but got none")
		}
		return statements.Statement[tree.Statement]{}, lexer.lastError
	}
	if lexer.stmt == nil {
		return statements.Statement[tree.Statement]{}, errors.AssertionFailedf("expected statement but got none")
	}
	return statements.Statement[tree.Statement]{
		AST: lexer.stmt,
		SQL: sql,
	}, nil
}
