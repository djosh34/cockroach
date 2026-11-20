// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package kvtenant

import "github.com/cockroachdb/cockroach/pkg/roachpb"

// rangeDescIterator is the concrete (private) implementation of the
// rangedesc.Iterator interface used by the connector.
type rangeDescIterator struct {
	rangeDescs []roachpb.RangeDescriptor
	curIdx     int
}

// Valid implements the rangedesc.Iterator interface.
func (i *rangeDescIterator) Valid() bool {
	return i.curIdx < len(i.rangeDescs)
}

// Next implements the rangedesc.Iterator interface.
func (i *rangeDescIterator) Next() {
	i.curIdx++
}

// CurRangeDescriptor implements the rangedesc.Iterator interface.
func (i *rangeDescIterator) CurRangeDescriptor() roachpb.RangeDescriptor {
	return i.rangeDescs[i.curIdx]
}
