// Copyright 2023 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package sqlutils

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/cockroachdb/cockroach/pkg/sql/parser"
	"github.com/cockroachdb/cockroach/pkg/sql/pgwire/pgerror"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
)

func parseOne(t *testing.T, input string) (tree.NodeFormatter, error) {
	t.Helper()
	stmt, err := parser.ParseOne(input)
	if err != nil {
		return nil, err
	}
	return stmt.AST, err
}

// VerifyParseFormat is used in the SQL datadriven parser tests to check that a
// successfully parsed expression round trips and correctly handles formatting
// flags.
func VerifyParseFormat(t *testing.T, input string) string {
	t.Helper()

	// Check parse.
	stmts, err := parser.Parse(input)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	// Check pretty-print roundtrip.
	VerifyStatementPrettyRoundtrip(t, input)

	ref := stmts.StringWithFlags(tree.FmtSimple)
	note := ""
	if ref != input {
		note = " -- normalized!"
	}

	// Check roundtrip and formatting with flags.
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%s%s\n", ref, note)
	fmt.Fprintln(&buf, stmts.StringWithFlags(tree.FmtAlwaysGroupExprs), "-- fully parenthesized")
	constantsHidden := stmts.StringWithFlags(tree.FmtHideConstants)
	fmt.Fprintln(&buf, constantsHidden, "-- literals removed")

	// As of this writing, the SQL statement stats proceed as follows:
	// first the literals are removed from statement to form a stat key,
	// then the stat key is re-parsed, to undergo the anonymization stage.
	// We also want to check the re-parsing is fine.
	reparsedStmts, err := parser.Parse(constantsHidden)
	if err != nil {
		t.Fatalf("unexpected error when reparsing without literals: %+v", err)
	} else {
		reparsedStmtsS := reparsedStmts.String()
		if reparsedStmtsS != constantsHidden {
			t.Fatalf(
				"mismatched AST when reparsing without literals:\noriginal: %s\nexpected: %s\nactual:   %s",
				input, constantsHidden, reparsedStmtsS,
			)
		}
	}

	fmt.Fprintln(&buf, stmts.StringWithFlags(tree.FmtAnonymize), "-- identifiers removed")
	if strings.Contains(ref, tree.PasswordSubstitution) {
		fmt.Fprintln(&buf, stmts.StringWithFlags(tree.FmtShowPasswords), "-- passwords exposed")
	}

	return buf.String()
}

var issueLinkRE = regexp.MustCompile("https://go.crdb.dev/issue-v/([0-9]+)/.*")

// VerifyParseError is used in the SQL datadriven parser tests to
// check that an unsuccessfully parsed expression returns an expected error.
func VerifyParseError(err error) string {
	pgerr := pgerror.Flatten(err)
	msg := pgerr.Message
	if pgerr.Detail != "" {
		msg += "\nDETAIL: " + pgerr.Detail
	}
	if pgerr.Hint != "" {
		msg += "\nHINT: " + pgerr.Hint
	}
	msg = issueLinkRE.ReplaceAllString(msg, "https://go.crdb.dev/issue-v/$1/")
	return msg
}
