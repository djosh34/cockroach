// Copyright 2020 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package cmpconn

import (
	"math/big"
	"strings"

	"github.com/cockroachdb/apd/v3"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/util/duration"
	"github.com/cockroachdb/errors"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/jackc/pgx/v5/pgtype"
)

// CompareVals returns an error if a and b differ, specifying what the
// difference is. It is designed to compare SQL results from a query
// executed on two different servers or configurations (i.e., cockroach and
// postgres). Postgres and Cockroach have subtle differences in their result
// types and OIDs. This function is aware of those and is able to correctly
// compare those values.
func CompareVals(a, b []interface{}) error {
	if len(a) != len(b) {
		return errors.Errorf("size difference: %d != %d", len(a), len(b))
	}
	if len(a) == 0 {
		return nil
	}
	if diff := cmp.Diff(a, b, cmpOptions...); diff != "" {
		return errors.Newf("unexpected diff:\n%s", diff)
	}
	return nil
}

var (
	cmpOptions = []cmp.Option{
		cmp.Transformer("", func(x []interface{}) []interface{} {
			out := make([]interface{}, len(x))
			for i, v := range x {
				switch t := v.(type) {
				case []interface{}:
					// Postgres and Cockroach differ in the element types of empty
					// arrays, so all empty arrays are equal.
					if len(t) == 0 {
						v = nil
					}
				case pgtype.Bits:
					if t.Valid {
						v = bitString(t)
					}
				case pgtype.Interval:
					if t.Valid {
						v = duration.DecodeDuration(int64(t.Months), int64(t.Days), t.Microseconds*1000)
					}
				case string:
					// Postgres sometimes adds spaces to the end of a string.
					t = strings.TrimSpace(t)
					v = strings.Replace(t, "T00:00:00+00:00", "T00:00:00Z", 1)

					// Postgres only shows the minutes offset of a timezone if it is
					// non-zero.
					// See https://github.com/cockroachdb/cockroach/issues/41563
					v = strings.Replace(t, ":00+00:00", ":00+00", 1)
				case pgtype.Numeric:
					if t.Valid {
						if t.NaN {
							v = &apd.Decimal{Form: apd.NaN}
						} else {
							var coeff apd.BigInt
							coeff.SetMathBigInt(t.Int)
							v = apd.NewWithBigInt(&coeff, t.Exp)
						}
					}
				case int64:
					v = apd.New(t, 0)
				}
				out[i] = v
			}
			return out
		}),

		cmpopts.EquateEmpty(),
		cmpopts.EquateNaNs(),
		cmpopts.EquateApprox(0.00001, 0),
		cmp.Comparer(func(x, y *big.Int) bool {
			return x.Cmp(y) == 0
		}),
		cmp.Comparer(func(x, y *apd.Decimal) bool {
			var a, b, min, sub apd.Decimal
			a.Abs(x)
			b.Abs(y)
			if a.Cmp(&b) > 0 {
				min.Set(&b)
			} else {
				min.Set(&a)
			}
			ctx := tree.DecimalCtx
			_, _ = ctx.Mul(&min, &min, decimalCloseness)
			_, _ = ctx.Sub(&sub, x, y)
			sub.Abs(&sub)
			return sub.Cmp(&min) <= 0
		}),
		cmp.Comparer(func(x, y duration.Duration) bool {
			return x.Compare(y) == 0
		}),
	}
	decimalCloseness = apd.New(1, -6)
)

// bitString returns the bits of b as a string of 0s and 1s.
func bitString(b pgtype.Bits) string {
	var sb strings.Builder
	for i := int32(0); i < b.Len; i++ {
		if b.Bytes[i/8]&(0x80>>(i%8)) != 0 {
			sb.WriteByte('1')
		} else {
			sb.WriteByte('0')
		}
	}
	return sb.String()
}
