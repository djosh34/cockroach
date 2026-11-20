// Copyright 2021 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package scbuild

import (
	"context"
	crypto_rand "crypto/rand"

	"github.com/cockroachdb/cockroach/pkg/sql/faketreeeval"
	"github.com/cockroachdb/cockroach/pkg/sql/schemachanger/scbuild/internal/scbuildstmt"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/eval"
	"github.com/cockroachdb/cockroach/pkg/sql/sem/tree"
	"github.com/cockroachdb/cockroach/pkg/sql/sessiondata"
	"github.com/cockroachdb/cockroach/pkg/util/ulid"
)

var _ scbuildstmt.TreeContextBuilder = buildCtx{}

// SemaCtx implements the scbuildstmt.TreeContextBuilder interface.
func (b buildCtx) SemaCtx() *tree.SemaContext {
	return newSemaCtx(b.Dependencies)
}

func newSemaCtx(d Dependencies) *tree.SemaContext {
	semaCtx := tree.MakeSemaContext()
	semaCtx.Annotations = nil
	semaCtx.SearchPath = &d.SessionData().SearchPath
	semaCtx.TypeResolver = d.CatalogReader()
	semaCtx.FunctionResolver = d.CatalogReader()
	semaCtx.NameResolver = d.CatalogReader()
	semaCtx.DateStyle = d.SessionData().GetDateStyle()
	semaCtx.IntervalStyle = d.SessionData().GetIntervalStyle()
	semaCtx.UnsupportedTypeChecker = eval.NewUnsupportedTypeChecker(d.ClusterSettings().Version)
	return &semaCtx
}

// EvalCtx implements the scbuildstmt.TreeContextBuilder interface.
func (b buildCtx) EvalCtx() *eval.Context {
	return newEvalCtx(b.Context, b.Dependencies)
}

func newEvalCtx(ctx context.Context, d Dependencies) *eval.Context {
	evalCtx := &eval.Context{
		ClusterID:          d.ClusterID(),
		SessionDataStack:   sessiondata.NewStack(d.SessionData()),
		Planner:            &faketreeeval.DummyEvalPlanner{},
		PrivilegedAccessor: &faketreeeval.DummyPrivilegedAccessor{},
		SessionAccessor:    &faketreeeval.DummySessionAccessor{},
		ClientNoticeSender: d.ClientNoticeSender(),
		Sequence:           &faketreeeval.DummySequenceOperators{},
		Tenant:             &faketreeeval.DummyTenantOperator{},
		Settings:           d.ClusterSettings(),
		Codec:              d.Codec(),
		DescIDGenerator:    d.DescIDGenerator(),
		ULIDEntropy:        ulid.Monotonic(crypto_rand.Reader, 0),
	}
	evalCtx.SetDeprecatedContext(ctx)
	return evalCtx
}
