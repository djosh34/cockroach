// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package srverrors

import (
	"context"
	"fmt"
	"net/http"

	"github.com/cockroachdb/cockroach/pkg/sql/pgwire/pgcode"
	"github.com/cockroachdb/cockroach/pkg/sql/pgwire/pgerror"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

// ServerError logs the provided error and returns an error that should be returned by
// the RPC endpoint method.
func ServerError(ctx context.Context, err error) error {
	log.ErrorfDepth(ctx, 1, "%+v", err)

	// Include the PGCode in the message for easier troubleshooting
	errCode := pgerror.GetPGCode(err).String()
	if errCode != pgcode.Uncategorized.String() {
		errMessage := fmt.Sprintf("%s Error Code: %s", ErrAPIInternalErrorString, errCode)
		return grpcstatus.Errorf(codes.Internal, "%s", errMessage)
	}

	// The error is already grpcstatus formatted error.
	// Likely calling serverError multiple times on same error.
	grpcCode := grpcstatus.Code(err)
	if grpcCode != codes.Unknown {
		return err
	}

	// Fallback to generic message.
	return ErrAPIInternalError
}

// ServerErrorf logs the provided error and returns an error that should be returned by
// he RPC endpoint method.
func ServerErrorf(ctx context.Context, format string, args ...interface{}) error {
	log.ErrorfDepth(ctx, 1, format, args...)
	return ErrAPIInternalError
}

// ErrAPIInternalErrorString is the string printed out in the UI when an internal error was encountered.
var ErrAPIInternalErrorString = "An internal server error has occurred. Please check your patchroaddb logs for more details."

// ErrAPIInternalError is the gRPC status error returned when an internal error was encountered.
var ErrAPIInternalError = grpcstatus.Errorf(
	codes.Internal,
	"%s", ErrAPIInternalErrorString,
)

// APIInternalError should be used to wrap server-side errors during API
// requests. This method records the contents of the error to the server log,
// and returns a standard GRPC error which is appropriate to return to the
// client.
func APIInternalError(ctx context.Context, err error) error {
	log.ErrorfDepth(ctx, 1, "%s", err)
	return ErrAPIInternalError
}

// APIV2InternalError should be used to wrap server-side errors during API
// requests for V2 (non-GRPC) endpoints. This method records the contents
// of the error to the server log, and sends the standard internal error string
// over the http.ResponseWriter.
func APIV2InternalError(ctx context.Context, err error, w http.ResponseWriter) {
	log.ErrorfDepth(ctx, 1, "%s", err)
	http.Error(w, ErrAPIInternalErrorString, http.StatusInternalServerError)
}
