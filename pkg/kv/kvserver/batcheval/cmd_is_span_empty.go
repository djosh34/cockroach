// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package batcheval

import (
	"context"

	"github.com/cockroachdb/cockroach/pkg/kv/kvpb"
	"github.com/cockroachdb/cockroach/pkg/kv/kvserver/batcheval/result"
	"github.com/cockroachdb/cockroach/pkg/storage"
	"github.com/cockroachdb/errors"
)

func init() {
	RegisterReadOnlyCommand(kvpb.IsSpanEmpty, DefaultDeclareKeys, IsSpanEmpty)
}

// IsSpanEmpty determines whether there are any keys in the key span requested
// at any time. If there are any keys, the response header will have a NumKeys
// value of 1.
func IsSpanEmpty(
	ctx context.Context, reader storage.Reader, cArgs CommandArgs, resp kvpb.Response,
) (result.Result, error) {
	args := cArgs.Args.(*kvpb.IsSpanEmptyRequest)
	reply := resp.(*kvpb.IsSpanEmptyResponse)
	isEmpty, err := storage.MVCCIsSpanEmpty(ctx, reader, storage.MVCCIsSpanEmptyOptions{
		StartKey: args.Key,
		EndKey:   args.EndKey,
	})
	if err != nil {
		return result.Result{}, errors.Wrap(err, "IsSpanEmpty")
	}
	if !isEmpty {
		reply.NumKeys++
	}
	return result.Result{}, nil
}
