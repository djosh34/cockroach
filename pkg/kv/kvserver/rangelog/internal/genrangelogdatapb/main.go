// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

// Command genrangelogdata takes csv rangelog data that might have
// come from a sql query and converts it to binary encoded protobuf
// data.
//
// The reason for this is so that the testdata remains robust to field
// renames.
package main

import (
	"fmt"
	"os"

	"github.com/cockroachdb/cockroach/pkg/cli/exit"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/rangelog/internal/rangelogtestpb"
	"github.com/cockroachdb/cockroach/pkg/util/protoutil"
	"github.com/cockroachdb/errors"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		exit.WithCode(exit.UnspecifiedError())
	}
}

func run(args []string) error {
	if len(args) != 2 {
		return errors.Errorf("requires 2 argument, got %d", len(args))
	}
	input, output := args[0], args[1]
	data, err := os.ReadFile(input)
	if err != nil {
		return errors.Wrapf(err, "failed to read csv file %v", args[0])
	}
	parsed, err := rangelogtestpb.ParseCSV(data)
	if err != nil {
		return errors.Wrap(err, "failed to parse input")
	}
	encoded, err := protoutil.Marshal(parsed)
	if err != nil {
		return errors.Wrap(err, "failed to encode output")
	}
	return errors.Wrap(os.WriteFile(output, encoded, 0666),
		"failed to write output")
}
