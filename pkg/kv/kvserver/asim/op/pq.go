// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package op

type queuedOp struct {
	ControlledOperation
	seq   int
	index int
}

type priorityQueue struct {
	seqGen int
	items  []*queuedOp
}

// Less is part of the container.Heap interface.
func (pq *priorityQueue) Less(i, j int) bool {
	a, b := pq.items[i], pq.items[j]
	if a.Next().Equal(b.Next()) {
		return a.seq < b.seq
	}
	return a.Next().Before(b.Next())
}

// Swap is part of the container.Heap interface.
func (pq *priorityQueue) Swap(i, j int) {
	pq.items[i], pq.items[j] = pq.items[j], pq.items[i]
	pq.items[i].index, pq.items[j].index = i, j
}

// Push is part of the container.Heap interface.
func (pq *priorityQueue) Push(x interface{}) {
	n := len(pq.items)
	item := x.(*queuedOp)
	item.index = n
	pq.seqGen++
	item.seq = pq.seqGen
	pq.items = append(pq.items, item)
}

// Pop is part of the container.Heap interface.
func (pq *priorityQueue) Pop() interface{} {
	old := pq.items
	n := len(old)
	item := old[n-1]
	item.index = -1 // for safety
	old[n-1] = nil  // for gc
	pq.items = old[0 : n-1]

	return item
}

// Len is part of the container.Heap interface.
func (pq *priorityQueue) Len() int {
	return len(pq.items)
}
