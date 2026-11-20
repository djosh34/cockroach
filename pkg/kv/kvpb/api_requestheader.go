// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package kvpb

import "github.com/cockroachdb/cockroach/pkg/util/buildutil"

func (r *RequestHeader) pure() RequestHeaderPure {
	return RequestHeaderPure{
		Key:      r.Key,
		EndKey:   r.EndKey,
		Sequence: r.Sequence,
	}
}

func (r *RequestHeader) crdbTest() RequestHeaderCrdbTest {
	return RequestHeaderCrdbTest(*r)
}

// Size implements protoutil.Message.
func (r *RequestHeader) Size() int {
	if buildutil.CrdbTestBuild && r.KVNemesisSeq.Get() != 0 {
		rt := r.crdbTest() //gcassert:noescape
		return rt.Size()
	}
	p := r.pure() //gcassert:noescape
	return p.Size()
}

// Marshal implements protoutil.Message.
func (r *RequestHeader) Marshal() ([]byte, error) {
	if buildutil.CrdbTestBuild && r.KVNemesisSeq.Get() != 0 {
		rt := r.crdbTest()
		return rt.Marshal()
	}
	p := r.pure()
	return p.Marshal()
}

// MarshalTo implements protoutil.Message.
func (r *RequestHeader) MarshalTo(buf []byte) (int, error) {
	if buildutil.CrdbTestBuild && r.KVNemesisSeq.Get() != 0 {
		rt := r.crdbTest() //gcassert:noescape
		return rt.MarshalTo(buf)
	}
	p := r.pure() //gcassert:noescape
	return p.MarshalTo(buf)
}

// MarshalToSizedBuffer implements protoutil.Message.
func (r *RequestHeader) MarshalToSizedBuffer(buf []byte) (int, error) {
	if buildutil.CrdbTestBuild && r.KVNemesisSeq.Get() != 0 {
		rt := r.crdbTest() //gcassert:noescape
		return rt.MarshalToSizedBuffer(buf)
	}
	p := r.pure() //gcassert:noescape
	return p.MarshalToSizedBuffer(buf)
}
