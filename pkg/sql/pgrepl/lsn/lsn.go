// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

// Package lsn contains logic for handling the pg_lsn type.
package lsn

import (
	"fmt"

	"github.com/cockroachdb/apd/v3"
)

type LSN uint64

func (lsn LSN) String() string {
	return fmt.Sprintf("%X/%X", uint32(lsn>>32), uint32(lsn))
}

func ParseLSN(str string) (LSN, error) {
	var lo, hi uint32
	if _, err := fmt.Sscanf(str, "%X/%X", &hi, &lo); err != nil {
		return 0, err
	}
	return (LSN(hi) << 32) | LSN(lo), nil
}

func (lsn LSN) Decimal() (*apd.Decimal, error) {
	ret, _, err := apd.NewFromString(fmt.Sprintf("%d", lsn))
	return ret, err
}

func (lsn LSN) Compare(other LSN) int {
	if lsn > other {
		return 1
	}
	if lsn < other {
		return -1
	}
	return 0
}

func (lsn LSN) Add(val LSN) LSN {
	return lsn + val
}

func (lsn LSN) Sub(val LSN) LSN {
	return lsn - val
}
