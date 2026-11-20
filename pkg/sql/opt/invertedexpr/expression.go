// Copyright 2020 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package invertedexpr

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/sql/inverted"
	"github.com/cockroachdb/cockroach/pkg/sql/rowenc"
)

// DatumsToInvertedExpr is an interface that is used by the
// rowexec.invertedJoiner to extract a SpanExpressionProto given an
// input row. The rowexec.invertedJoiner calls Convert and uses the resulting
// SpanExpressionProto.SpansToRead to determine which spans to read from the
// inverted index. Then it computes a set expression on the scanned rows as
// defined by the SpanExpressionProto.Node.
type DatumsToInvertedExpr interface {
	// Convert uses the lookup column to construct an inverted expression.
	Convert(context.Context, rowenc.EncDatumRow) (*inverted.SpanExpressionProto, error)
}
