// Copyright 2020 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

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
