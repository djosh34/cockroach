// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package spanconfigreporter

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/roachpb"
	"github.com/cockroachdb/cockroach/pkg/spanconfig"
	"github.com/cockroachdb/errors"
)

// DisabledReporter is a spanconfig.Reporter that only returns "disabled"
// errors.
var DisabledReporter = disabled{}

type disabled struct{}

var _ spanconfig.Reporter = disabled{}

// SpanConfigConformance implements the spanconfig.Reporter interface.
func (d disabled) SpanConfigConformance(
	ctx context.Context, spans []roachpb.Span,
) (roachpb.SpanConfigConformanceReport, error) {
	return roachpb.SpanConfigConformanceReport{}, errors.New("span configs disabled")
}
