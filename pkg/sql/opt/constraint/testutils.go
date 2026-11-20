// Copyright 2018 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package constraint

import (
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/cockroach/pkg/sql/opt"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/eval"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
	"github.com/cockroachdb/errors"
)

// ParseConstraint parses a constraint in the format of Constraint.String, e.g:
//
//	"/1/2/3: [/1 - /2]".
func ParseConstraint(evalCtx *eval.Context, str string) Constraint {
	s := strings.SplitN(str, ": ", 2)
	if len(s) != 2 {
		panic(errors.AssertionFailedf("invalid constraint format: %s", str))
	}
	var cols []opt.OrderingColumn
	for _, v := range parseIntPath(s[0]) {
		cols = append(cols, opt.OrderingColumn(v))
	}
	var c Constraint
	c.Columns.Init(cols)
	c.Spans = parseSpans(evalCtx, s[1])
	return c
}

// parseSpans parses a list of spans with integer values like:
//
//	"[/1 - /2] [/5 - /6]".
func parseSpans(evalCtx *eval.Context, str string) Spans {
	if str == "" || str == "contradiction" {
		return Spans{}
	}
	if str == "unconstrained" {
		s := Spans{}
		s.InitSingleSpan(&UnconstrainedSpan)
		return s
	}
	s := strings.Split(str, " ")
	// Each span has three pieces.
	if len(s)%3 != 0 {
		panic(errors.AssertionFailedf("invalid span format: %s", str))
	}
	var result Spans
	for i := 0; i < len(s)/3; i++ {
		sp := ParseSpan(evalCtx, strings.Join(s[i*3:i*3+3], " "))
		result.Append(&sp)
	}
	return result
}

// ParseSpan parses a span in the format of Span.String, e.g: [/1 - /2].
// If no types are passed in, the type is inferred as being an int if possible;
// otherwise a string. If any types are specified, they must be specified for
// every datum.
func ParseSpan(evalCtx *eval.Context, str string, typs ...types.Family) Span {
	if len(str) < len("[ - ]") {
		panic(str)
	}
	boundary := map[byte]SpanBoundary{
		'[': IncludeBoundary,
		']': IncludeBoundary,
		'(': ExcludeBoundary,
		')': ExcludeBoundary,
	}
	s, e := str[0], str[len(str)-1]
	if (s != '[' && s != '(') || (e != ']' && e != ')') {
		panic(str)
	}
	keys := strings.Split(str[1:len(str)-1], " - ")
	if len(keys) != 2 {
		panic(str)
	}

	// Retrieve the values of the longest key.
	longestKey := tree.ParsePath(keys[0])
	endDatums := tree.ParsePath(keys[1])
	if len(longestKey) < len(endDatums) {
		longestKey = endDatums
	}
	if len(longestKey) > 0 && len(typs) == 0 {
		// Infer the datum types and populate typs accordingly.
		typs = tree.InferTypes(longestKey)
	}

	var sp Span
	startVals := parseDatumPath(evalCtx, keys[0], typs)
	endVals := parseDatumPath(evalCtx, keys[1], typs)
	sp.Init(
		MakeCompositeKey(startVals...), boundary[s],
		MakeCompositeKey(endVals...), boundary[e],
	)
	return sp
}

// parseIntPath parses a string like "/1/2/3" into a list of integers.
func parseIntPath(str string) []int {
	var res []int
	for _, valStr := range tree.ParsePath(str) {
		val, err := strconv.Atoi(valStr)
		if err != nil {
			panic(err)
		}
		res = append(res, val)
	}
	return res
}

// parseDatumPath parses a span key string like "/1/2/3".
// Only NULL and a subset of types are currently supported.
func parseDatumPath(evalCtx *eval.Context, str string, typs []types.Family) []tree.Datum {
	var res []tree.Datum
	for i, valStr := range tree.ParsePath(str) {
		if i >= len(typs) {
			panic(errors.AssertionFailedf("invalid types"))
		}

		if valStr == "NULL" {
			res = append(res, tree.DNull)
			continue
		}
		var val tree.Datum
		var err error
		switch typs[i] {
		case types.BoolFamily:
			val, err = tree.ParseDBool(valStr)
		case types.IntFamily:
			val, err = tree.ParseDInt(valStr)
		case types.FloatFamily:
			val, err = tree.ParseDFloat(valStr)
		case types.DecimalFamily:
			val, err = tree.ParseDDecimal(valStr)
		case types.DateFamily:
			val, _, err = tree.ParseDDate(evalCtx, valStr)
		case types.TimestampFamily:
			val, _, err = tree.ParseDTimestamp(evalCtx, valStr, time.Microsecond)
		case types.TimestampTZFamily:
			val, _, err = tree.ParseDTimestampTZ(evalCtx, valStr, time.Microsecond)
		case types.StringFamily:
			val = tree.NewDString(valStr)
		case types.BytesFamily:
			val = tree.NewDBytes(tree.DBytes(valStr))
		case types.OidFamily:
			var dInt *tree.DInt
			dInt, err = tree.ParseDInt(valStr)
			if err == nil {
				val, err = tree.IntToOid(*dInt)
			}
		case types.UuidFamily:
			val, err = tree.ParseDUuidFromString(valStr)
		case types.INetFamily:
			val, err = tree.ParseDIPAddrFromINetString(valStr)
		case types.TimeFamily:
			val, _, err = tree.ParseDTime(evalCtx, valStr, time.Microsecond)
		case types.TimeTZFamily:
			val, _, err = tree.ParseDTimeTZ(evalCtx, valStr, time.Microsecond)
		default:
			panic(errors.AssertionFailedf("type %s not supported", typs[i].String()))
		}
		if err != nil {
			panic(err)
		}
		res = append(res, val)
	}
	return res
}
