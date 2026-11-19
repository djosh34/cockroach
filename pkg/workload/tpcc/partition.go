// Copyright 2018 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package tpcc

import (
	"bytes"
	gosql "database/sql"
	"fmt"
	"strings"

	"github.com/cockroachdb/errors"
	"golang.org/x/exp/rand"
)

// partitioner encapsulates all logic related to partitioning discrete numbers
// of warehouses into disjoint sets of roughly equal sizes. Partitions are then
// evenly assigned "active" warehouses, which allows for an even split of live
// warehouses across partitions without the need to repartition when the active
// count is changed.
type partitioner struct {
	total  int // e.g. the total number of warehouses
	active int // e.g. the active number of warehouses
	parts  int // the number of partitions to break `total` into

	partBounds   []int       // the boundary points between partitions
	partElems    [][]int     // the elements active in each partition
	partElemsMap map[int]int // mapping from element to partition index
	totalElems   []int       // all active elements
}

func makePartitioner(total, active, parts int) (*partitioner, error) {
	if total <= 0 {
		return nil, errors.Errorf("total must be positive; %d", total)
	}
	if active <= 0 {
		return nil, errors.Errorf("active must be positive; %d", active)
	}
	if parts <= 0 {
		return nil, errors.Errorf("parts must be positive; %d", parts)
	}
	if active > total {
		return nil, errors.Errorf("active > total; %d > %d", active, total)
	}
	if parts > total {
		return nil, errors.Errorf("parts > total; %d > %d", parts, total)
	}

	// Partition boundary points.
	//
	// bounds contains the boundary points between partitions, where each point
	// in the slice corresponds to the exclusive end element of one partition
	// and and the inclusive start element of the next.
	//
	//  total  = 20
	//  parts  = 3
	//  bounds = [0, 6, 13, 20]
	//
	bounds := make([]int, parts+1)
	for i := range bounds {
		bounds[i] = (i * total) / parts
	}

	// Partition sizes.
	//
	// sizes contains the number of elements that are active in each partition.
	//
	//  active = 10
	//  parts  = 3
	//  sizes  = [3, 3, 4]
	//
	sizes := make([]int, parts)
	for i := range sizes {
		s := (i * active) / parts
		e := ((i + 1) * active) / parts
		sizes[i] = e - s
	}

	// Partitions.
	//
	// partElems enumerates the active elements in each partition.
	//
	//  total     = 20
	//  active    = 10
	//  parts     = 3
	//  partElems = [[0, 1, 2], [6, 7, 8], [13, 14, 15, 16]]
	//
	partElems := make([][]int, parts)
	for i := range partElems {
		partAct := make([]int, sizes[i])
		for j := range partAct {
			partAct[j] = bounds[i] + j
		}
		partElems[i] = partAct
	}

	// Partition reverse mapping.
	//
	// partElemsMap maps each active element to its partition index.
	//
	//  total        = 20
	//  active       = 10
	//  parts        = 3
	//  partElemsMap = {0:0, 1:0, 2:0, 6:1, 7:1, 8:1, 13:2, 14:2, 15:2, 16:2}
	//
	partElemsMap := make(map[int]int)
	for p, elems := range partElems {
		for _, elem := range elems {
			partElemsMap[elem] = p
		}
	}

	// Total elements.
	//
	// totalElems aggregates all active elements into a single slice.
	//
	//  total      = 20
	//  active     = 10
	//  parts      = 3
	//  totalElems = [0, 1, 2, 6, 7, 8, 13, 14, 15, 16]
	//
	var totalElems []int
	for _, elems := range partElems {
		totalElems = append(totalElems, elems...)
	}

	return &partitioner{
		total:  total,
		active: active,
		parts:  parts,

		partBounds:   bounds,
		partElems:    partElems,
		partElemsMap: partElemsMap,
		totalElems:   totalElems,
	}, nil
}

// randActive returns a random active element.
func (p *partitioner) randActive(rng *rand.Rand) int {
	return p.totalElems[rng.Intn(len(p.totalElems))]
}
