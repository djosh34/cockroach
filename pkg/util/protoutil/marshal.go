// Copyright 2016 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package protoutil

import (
	"fmt"

	"github.com/cockroachdb/cockroach/pkg/util/buildutil"
	"github.com/gogo/protobuf/proto"
)

// Message extends the proto.Message interface with the MarshalTo and Size
// methods we tell gogoproto to generate for us.
type Message interface {
	proto.Message
	MarshalTo(data []byte) (int, error)
	MarshalToSizedBuffer(data []byte) (int, error)
	Unmarshal(data []byte) error
	Size() int
}

// Marshal encodes pb into the wire format and returns the resulting byte slice.
func Marshal(pb Message) ([]byte, error) {
	dest := make([]byte, pb.Size())
	if _, err := MarshalToSizedBuffer(pb, dest); err != nil {
		return nil, err
	}
	return dest, nil
}

// MarshalTo encodes pb into the wire format and writes the result into the
// provided byte slice, returning the number of bytes written.
//
// dest is required to have a capacity of at least pb.Size() bytes.
func MarshalTo(pb Message, dest []byte) (int, error) {
	if buildutil.CrdbTestBuild {
		if pb.Size() > cap(dest) {
			panic(fmt.Sprintf("MarshalTo called for %T with slice with insufficient "+
				"capacity: pb.Size()=%d, cap(dest)=%d", pb, pb.Size(), cap(dest)))
		}
	}
	return pb.MarshalTo(dest)
}

// MarshalToSizedBuffer encodes pb into the wire format and writes the result
// into the provided byte slice, returning the number of bytes written.
//
// dest is required to have a length of exactly pb.Size() bytes.
func MarshalToSizedBuffer(pb Message, dest []byte) (int, error) {
	if buildutil.CrdbTestBuild {
		if pb.Size() != len(dest) {
			panic(fmt.Sprintf("MarshalToSizedBuffer called for %T with slice with "+
				"incorrect length: pb.Size()=%d, len(dest)=%d", pb, pb.Size(), len(dest)))
		}
	}
	return pb.MarshalToSizedBuffer(dest)
}

// Unmarshal parses the protocol buffer representation in buf and places the
// decoded result in pb. If the struct underlying pb does not match the data in
// buf, the results can be unpredictable.
//
// Unmarshal resets pb before starting to unmarshal, so any existing data in pb
// is always removed.
func Unmarshal(data []byte, pb Message) error {
	pb.Reset()
	return pb.Unmarshal(data)
}
