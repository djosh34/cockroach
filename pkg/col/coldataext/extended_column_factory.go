// Copyright 2020 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package coldataext

import (
	"github.com/cockroachdb/cockroach/pkg/col/coldata"
	"github.com/cockroachdb/cockroach/pkg/col/typeconv"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/eval"
	"github.com/cockroachdb/cockroach/pkg/sql/types"
)

// extendedColumnFactory stores an evalCtx which can be used to construct
// datumVec later. This is to prevent plumbing evalCtx to all vectorized
// operators as well as avoiding introducing dependency from coldata on tree
// package.
type extendedColumnFactory struct {
	evalCtx *eval.Context
}

var _ coldata.ColumnFactory = &extendedColumnFactory{}

// NewExtendedColumnFactory returns an extendedColumnFactory instance.
func NewExtendedColumnFactory(evalCtx *eval.Context) coldata.ColumnFactory {
	return &extendedColumnFactory{evalCtx: evalCtx}
}

// NewExtendedColumnFactoryNoEvalCtx returns an extendedColumnFactory that will
// be producing coldata.DatumVecs that aren't fully initialized - the eval
// context is not set on those vectors. This can be acceptable if the caller
// cannot provide the eval.Context but also doesn't intend to compare datums.
func NewExtendedColumnFactoryNoEvalCtx() coldata.ColumnFactory {
	return &extendedColumnFactory{}
}

func (cf *extendedColumnFactory) MakeColumn(t *types.T, n int) coldata.Column {
	if typeconv.TypeFamilyToCanonicalTypeFamily(t.Family()) == typeconv.DatumVecCanonicalTypeFamily {
		return newDatumVec(t, n, cf.evalCtx)
	}
	return coldata.StandardColumnFactory.MakeColumn(t, n)
}
