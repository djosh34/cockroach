// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package enginepb

import "github.com/cockroachdb/cockroach/pkg/util/buildutil"

// IsEmpty returns true if the header is empty.
// gcassert:inline
func (h MVCCValueHeader) IsEmpty() bool {
	return h == MVCCValueHeader{}
}

func (h *MVCCValueHeader) pure() MVCCValueHeaderPure {
	return MVCCValueHeaderPure{
		LocalTimestamp:   h.LocalTimestamp,
		OmitInRangefeeds: h.OmitInRangefeeds,
	}
}

func (h *MVCCValueHeader) crdbTest() MVCCValueHeaderCrdbTest {
	return (MVCCValueHeaderCrdbTest)(*h)
}

// Size implements protoutil.Message.
func (h *MVCCValueHeader) Size() int {
	if buildutil.CrdbTestBuild && h.KVNemesisSeq.Get() != 0 {
		ht := h.crdbTest() //gcassert:noescape
		return ht.Size()
	}
	p := h.pure() //gcassert:noescape
	return p.Size()
}

// Marshal implements protoutil.Message.
func (h *MVCCValueHeader) Marshal() ([]byte, error) {
	if buildutil.CrdbTestBuild && h.KVNemesisSeq.Get() != 0 {
		ht := h.crdbTest() //gcassert:noescape
		return ht.Marshal()
	}
	p := h.pure() //gcassert:noescape
	return p.Marshal()
}

// MarshalTo implements protoutil.Message.
func (h *MVCCValueHeader) MarshalTo(buf []byte) (int, error) {
	if buildutil.CrdbTestBuild && h.KVNemesisSeq.Get() != 0 {
		ht := h.crdbTest() //gcassert:noescape
		return ht.MarshalTo(buf)
	}
	p := h.pure() //gcassert:noescape
	return p.MarshalTo(buf)
}

// MarshalToSizedBuffer implements protoutil.Message.
func (h *MVCCValueHeader) MarshalToSizedBuffer(buf []byte) (int, error) {
	if buildutil.CrdbTestBuild && h.KVNemesisSeq.Get() != 0 {
		ht := h.crdbTest() //gcassert:noescape
		return ht.MarshalToSizedBuffer(buf)
	}
	p := h.pure() //gcassert:noescape
	return p.MarshalToSizedBuffer(buf)
}
