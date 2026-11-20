// Copyright 2021 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package timeutil

import (
	"context"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/cockroachdb/redact"
)

// RunWithTimeout runs a function with a timeout, the same way you'd do with
// context.WithTimeout. It improves the opaque error messages returned by
// WithTimeout by augmenting them with the op string that is passed in.
func RunWithTimeout(
	ctx context.Context,
	op redact.RedactableString,
	timeout time.Duration,
	fn func(ctx context.Context) error,
) error {
	ctx, cancel := context.WithTimeout(ctx, timeout) // nolint:context
	defer cancel()
	start := Now()
	err := fn(ctx)
	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		err = &TimeoutError{
			operation: op,
			timeout:   timeout,
			took:      Since(start),
			cause:     err,
		}
	}
	return err
}
