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
	"slices"
	"testing"
)

func TestUtilToInterfaceSlice(t *testing.T) {
	type obj struct {
		id int
	}

	input := []obj{
		{1},
		{2},
		{3},
	}

	actual := ToInterfaceSlice(input)

	if len(actual) != len(input) {
		t.Errorf("incorrect response length, expected %d, found %d", len(input), len(actual))
	}

	for i, v := range actual {
		switch a := v.(type) {
		case obj:
			if a != input[i] {
				t.Errorf("mismatched instance at index %d, expected %v, found %v", i, input[i], a)
			}
		default:
			t.Errorf("unexpected value in index %d, expected %v, found %v", i, input[i], v)
		}
	}
}

func TestUtilToTypedSlice(t *testing.T) {
	type obj struct {
		id int
	}

	input := []interface{}{
		obj{1},
		obj{2},
		obj{3},
	}

	actual := ToTypedSlice[obj](input)
	if len(actual) != len(input) {
		t.Errorf("incorrect response length, expected %d, found %d", len(input), len(actual))
	}

	for i, v := range actual {
		if v != input[i] {
			t.Errorf("mismatched instance at index %d, expected %v, found %v", i, input[i], v)
		}
	}
}

func TestUtilDistinctSlice(t *testing.T) {
	type obj struct {
		id int
	}

	input := []obj{
		{1},
		{2},
		{2},
		{2},
		{3},
	}

	expected := []obj{
		{1},
		{2},
		{3},
	}

	actual := DistinctSlice(input)

	if !slices.Equal(expected, actual) {
		t.Errorf("incorrect response, expected %v, found %v", expected, actual)
	}
}
