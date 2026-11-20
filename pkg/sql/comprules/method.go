// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package comprules

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/sql/compengine"
)

func method(
	label string, fn func(context.Context, compengine.Context) (compengine.Rows, error),
) compengine.Method {
	return m{label, fn}
}

type m struct {
	label string
	fn    func(context.Context, compengine.Context) (compengine.Rows, error)
}

var _ compengine.Method = m{}

func (m m) Name() string { return m.label }
func (m m) Call(ctx context.Context, c compengine.Context) (compengine.Rows, error) {
	return m.fn(ctx, c)
}
