// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package obsutil

import (
	"context"
	"fmt"

	"github.com/cockroachdb/cockroach/pkg/obsservice/obslib"
	"github.com/cockroachdb/cockroach/pkg/obsservice/obspb"
)

// StdOutConsumer implements the EventConsumer interface and logs
// each event it receives to STDOUT, for testing purposes.
//
// StdOutConsumer is not intended for real-world use.
type StdOutConsumer struct{}

func (s StdOutConsumer) Consume(ctx context.Context, event *obspb.Event) error {
	fmt.Printf("StdOutConsumer - consumed event: %v\n", event)
	return nil
}

var _ obslib.EventConsumer = (*StdOutConsumer)(nil)
