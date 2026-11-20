// Copyright 2023 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may not use this
// file except in compliance with the License. You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under
// the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF
// ANY KIND, either express or implied. See the License for the specific language
// governing permissions and limitations under the License.

package bulk

// Iterator is an interface to iterate a collection of objects of type T.
type Iterator[T any] interface {
	// Valid must be called after any call to Next(). It returns (true, nil) if
	// the iterator points to a valid value, and (false, nil) if the iterator has
	// moved past the last value. It returns (false, err) if there is an error in
	// the iterator.
	Valid() (bool, error)

	// Value returns the current value. The returned value is only valid until the
	// next call to Next(). is only valid until the
	Value() T

	// Next advances the iterator to the next value.
	Next()

	// Close closes the iterator.
	Close()
}

// CollectToSlice iterates over all objects in iterator and collects them into a
// slice.
func CollectToSlice[T any](iterator Iterator[T]) ([]T, error) {
	var values []T
	for ; ; iterator.Next() {
		if ok, err := iterator.Valid(); err != nil {
			return nil, err
		} else if !ok {
			break
		}

		values = append(values, iterator.Value())
	}
	return values, nil
}
