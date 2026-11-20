// Copyright 2022 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package main

import "sort"

type stringSet map[string]struct{}

func (ss stringSet) add(s string) { ss[s] = struct{}{} }

func (ss stringSet) removeAll(other stringSet) {
	for s := range other {
		delete(ss, s)
	}
}

func (ss stringSet) addAll(other stringSet) {
	for s := range other {
		ss.add(s)
	}
}

func (ss stringSet) ordered() []string {
	list := make([]string, 0, len(ss))
	for s := range ss {
		list = append(list, s)
	}
	sort.Strings(list)
	return list
}

func (ss stringSet) contains(name string) bool {
	_, exists := ss[name]
	return exists
}

func (ss stringSet) intersection(other stringSet) stringSet {
	intersection := stringSet{}
	for s := range ss {
		if other.contains(s) {
			intersection.add(s)
		}
	}
	return intersection
}
