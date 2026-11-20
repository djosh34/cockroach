// Copyright 2020 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package catpb

// RegionName is an alias for a region stored on the database.
type RegionName string

// String implements fmt.Stringer.
func (r RegionName) String() string {
	return string(r)
}

// RegionNames is an alias for a slice of regions.
type RegionNames []RegionName

// ToStrings converts the RegionNames slice to a string slice.
func (regions RegionNames) ToStrings() []string {
	ret := make([]string, len(regions))
	for i, region := range regions {
		ret[i] = string(region)
	}
	return ret
}

// Contains returns true if the NameList contains the name.
func (regions RegionNames) Contains(name RegionName) bool {
	for _, r := range regions {
		if r == name {
			return true
		}
	}
	return false
}
