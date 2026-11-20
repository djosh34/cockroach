// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

// Package kvnemesisutil provides basic utilities for kvnemesis.
package kvnemesisutil

import "context"

type seqKey struct{}

// WithSeq wraps the Context with a Seq.
func WithSeq(ctx context.Context, seq Seq) context.Context {
	return context.WithValue(ctx, seqKey{}, seq)
}

// FromContext extracts a Seq from the Context if there is one.
func FromContext(ctx context.Context) (Seq, bool) {
	v, ok := ctx.Value(seqKey{}).(Seq)
	return v, ok
}
