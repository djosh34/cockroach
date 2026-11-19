// Copyright 2023 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package statements

import "github.com/cockroachdb/cockroach/pkg/sql/sem/tree"

type AST interface {
}

// Statement is the result of parsing a single statement. It contains the AST
// node along with other information.
type Statement[T any] struct {
	// AST is the root of the AST tree for the parsed statement.
	// Note that it is NOT SAFE to access this currently with statement execution,
	// as unfortunately the AST is not immutable.
	// See issue https://github.com/cockroachdb/cockroach/issues/22847 for more
	// details on this problem.
	AST T

	// Comments is the list of parsed SQL comments.
	Comments []string

	// SQL is the original SQL from which the statement was parsed. Note that this
	// is not appropriate for use in logging, as it may contain passwords and
	// other sensitive data.
	SQL string

	// NumPlaceholders indicates the number of arguments to the statement (which
	// are referenced through placeholders). This corresponds to the highest
	// argument position (i.e. the x in "$x") that appears in the query.
	//
	// Note: where there are "gaps" in the placeholder positions, this number is
	// based on the highest position encountered. For example, for `SELECT $3`,
	// NumPlaceholders is 3. These cases are malformed and will result in a
	// type-check error.
	NumPlaceholders int

	// NumAnnotations indicates the number of annotations in the tree. It is equal
	// to the maximum annotation index.
	NumAnnotations tree.AnnotationIdx
}

// Statements is a list of parsed statements.
type Statements []Statement[tree.Statement]

// String returns the AST formatted as a string.
func (stmts Statements) String() string {
	return stmts.StringWithFlags(tree.FmtSimple)
}

// StringWithFlags returns the AST formatted as a string (with the given flags).
func (stmts Statements) StringWithFlags(flags tree.FmtFlags) string {
	ctx := tree.NewFmtCtx(flags)
	for i, s := range stmts {
		if i > 0 {
			ctx.WriteString("; ")
		}
		ctx.FormatNode(s.AST)
	}
	return ctx.CloseAndGetString()
}

type ParsedStmts interface {
	String() string
	StringWithFlags(flags tree.FmtFlags) string
}

var _ ParsedStmts = Statements{}
