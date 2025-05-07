// Copyright 2025 Metrostar Systems, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

// ToInterfaceSlice converts a typed slice to a slice of `interface{}`.
func ToInterfaceSlice[T any](s []T) []interface{} {
	res := make([]interface{}, len(s))

	for i, v := range s {
		res[i] = v
	}

	return res
}

// ToTypedSlice converts a slice of `interface{}` to a typed slice.
func ToTypedSlice[T any](s []interface{}) []T {
	res := make([]T, len(s))

	for i, v := range s {
		res[i] = v.(T)
	}

	return res
}

// DistinctSlice returns a new slice with duplicate entries removed.
// The input slice must contain elements of a comparable type.
func DistinctSlice[T comparable](s []T) []T {
	var res []T
	found := make(map[T]bool)

	for _, v := range s {
		if _, ok := found[v]; ok {
			continue
		}

		found[v] = true
		res = append(res, v)
	}

	return res
}
