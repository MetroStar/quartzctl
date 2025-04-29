package util

import "testing"

type testUtilValueOrDefaultWrapper[T comparable] struct {
	zero  T
	value T
	def   T
}

func testUtilValueOrDefaultCheck[T comparable](t *testing.T, tc testUtilValueOrDefaultWrapper[T]) {
	// passing the zero value for the type, should get the default back
	v1 := ValueOrDefault(tc.zero, tc.def)
	if v1 != tc.def {
		t.Errorf("failed to handle default case for type %T, expected %v, found %v", tc.def, tc.def, v1)
	}

	// passing an actual value, should get the same thing back
	v2 := ValueOrDefault(tc.value, tc.def)
	if v2 != tc.value {
		t.Errorf("failed to handle value case for type %T, expected %v, found %v", tc.value, tc.value, v2)
	}
}

func TestUtilValueOrDefault(t *testing.T) {
	val := struct{ id int }{id: 1}
	def := struct{ id int }{id: 2}

	// zero value, input, default
	test_cases := []interface{}{
		testUtilValueOrDefaultWrapper[int]{0, 10, 100},
		testUtilValueOrDefaultWrapper[string]{"", "foo", "bar"},
		testUtilValueOrDefaultWrapper[struct{ id int }]{struct{ id int }{}, val, def},
		testUtilValueOrDefaultWrapper[interface{}]{nil, &val, &def},
	}

	for _, tc := range test_cases {
		switch tcv := tc.(type) {
		case testUtilValueOrDefaultWrapper[int]:
			testUtilValueOrDefaultCheck(t, tcv)
		case testUtilValueOrDefaultWrapper[string]:
			testUtilValueOrDefaultCheck(t, tcv)
		case testUtilValueOrDefaultWrapper[struct{ id int }]:
			testUtilValueOrDefaultCheck(t, tcv)
		case testUtilValueOrDefaultWrapper[interface{}]:
			testUtilValueOrDefaultCheck(t, tcv)
		default:
			t.Errorf("unhandled test case type, %T", tcv)
		}
	}
}
