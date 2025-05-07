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

import (
	"maps"
	"slices"
	"sort"
)

// MergeMaps performs a shallow merge of two or more maps into a single map.
// Duplicate keys in subsequent maps will take precedence and overwrite earlier values.
func MergeMaps[K comparable, V interface{}](maps ...map[K]V) map[K]V {
	merged := make(map[K]V)

	for _, m := range maps {
		for k, v := range m {
			merged[k] = v
		}
	}

	return merged
}

// MapContainsKey checks if a map contains a specific key.
func MapContainsKey[K comparable, V interface{}](m map[K]V, key K) bool {
	_, ok := m[key]
	return ok
}

// MapIntKeysToSortedSlice converts a map with integer keys into a slice of values,
// sorted by the integer keys in ascending order.
func MapIntKeysToSortedSlice[V interface{}](m map[int]V) []V {
	var r []V
	ks := slices.Collect(maps.Keys(m))
	sort.Ints(ks)
	for _, k := range ks {
		r = append(r, m[k])
	}

	return r
}
