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
