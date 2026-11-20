// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package gcp

import (
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/googleapis/gax-go/v2/apierror"
	"github.com/stretchr/testify/assert"
	"google.golang.org/api/googleapi"
)

func TestErrorBehaviour(t *testing.T) {
	orig := &googleapi.Error{
		Code:    403,
		Message: "ACCESS DENIED. ALL YOUR BASE ARE BELONG TO US",
	}
	apiError, ok := apierror.ParseError(orig, false)
	if ok {
		orig.Wrap(apiError)
	}
	wrap1 := errors.Wrap(orig, "wrap1")
	wrap2 := errors.Wrap(wrap1, "wrap2")
	assert.Equal(t, "wrap1: googleapi: Error 403: ACCESS DENIED. ALL YOUR BASE ARE BELONG TO US", wrap1.Error())
	assert.Equal(t, "wrap2: wrap1: grpc: ACCESS DENIED. ALL YOUR BASE ARE BELONG TO US [code 7/PermissionDenied]", wrap2.Error())
}
