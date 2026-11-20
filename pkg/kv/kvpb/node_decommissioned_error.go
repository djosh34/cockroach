// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package kvpb

import (
	"github.com/cockroachdb/errors"
	_ "github.com/cockroachdb/errors/extgrpc" // for error encoder/decoder
	"github.com/cockroachdb/redact"
	ptypes "github.com/gogo/protobuf/types"
	gogostatus "github.com/gogo/status"
	"google.golang.org/grpc/codes"
)

// NewDecommissionedStatusErrorf returns a GRPC status with the given error code
// and formatted message whose detail is a *NodeDecommissionedError.
func NewDecommissionedStatusErrorf(errorCode codes.Code, format string, args ...interface{}) error {
	// Important: gogoproto ptypes and status, not google protobuf, see extgrpc pkg.
	st := gogostatus.Newf(errorCode, format, args...).Proto()
	det, err := ptypes.MarshalAny(&NodeDecommissionedError{})
	if err != nil {
		return err
	}
	st.Details = append(st.Details, det)
	return gogostatus.FromProto(st).Err()
}

func (err *NodeDecommissionedError) SafeFormatError(p errors.Printer) (next error) {
	p.Printf("node is decommissioned")
	return nil
}

func (err *NodeDecommissionedError) Error() string {
	return redact.Sprint(err).StripMarkers()
}

// IsDecommissionedStatusErr returns true if the error wraps a gRPC status error
// with a NodeDecommissionedError detail, i.e. it was created using
// NewDecommissionedStatusErrorf.
func IsDecommissionedStatusErr(err error) bool {
	s, ok := gogostatus.FromError(errors.UnwrapAll(err))
	if !ok {
		return false
	}
	for _, det := range s.Details() {
		if _, ok := det.(*NodeDecommissionedError); ok {
			return true
		}
	}
	return false
}
