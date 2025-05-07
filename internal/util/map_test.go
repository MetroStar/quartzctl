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

import "testing"

// https://dave.cheney.net/2013/06/30/how-to-write-benchmarks-in-go
var benchmarkResult map[string]string

func TestMergeMapsHappy(t *testing.T) {
	e1 := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	e2 := map[string]string{
		"key3": "value3",
		"key4": "value4",
	}

	actual := MergeMaps(e1, e2)

	for ek1, ev1 := range e1 {
		av, ok := actual[ek1]
		if !ok {
			t.Errorf("result map missing input key %s", ek1)
		} else if av != ev1 {
			t.Errorf("incorrect value for input key %s, expected %s, got %s", ek1, ev1, av)
		}
	}

	for ek2, ev2 := range e2 {
		av, ok := actual[ek2]
		if !ok {
			t.Errorf("result map missing input key %s", ek2)
		} else if av != ev2 {
			t.Errorf("incorrect value for input key %s, expected %s, got %s", ek2, ev2, av)
		}
	}
}

func TestMergeMapsOverwrite(t *testing.T) {
	testKey := "overwrite_me"
	ev := "something_new"

	e1 := map[string]string{
		testKey: "original",
	}
	e2 := map[string]string{
		testKey: ev,
	}

	actual := MergeMaps(e1, e2)

	av, ok := actual[testKey]
	if !ok {
		t.Errorf("result map missing input key %s", testKey)
	} else if av != ev {
		t.Errorf("incorrect value for input key %s, expected %s, got %s", testKey, ev, av)
	}
}

func BenchmarkMergeMaps(b *testing.B) {
	var r map[string]string

	for i := 0; i < b.N; i++ {
		e1 := map[string]string{
			"key1": "value1",
			"key2": "value2",
		}
		e2 := map[string]string{
			"key3": "value3",
			"key4": "value4",
		}

		r = MergeMaps(e1, e2)
	}

	benchmarkResult = r
}

func TestMapContainsKey(t *testing.T) {
	m := map[string]string{
		"key1": "value1",
	}

	if MapContainsKey(m, "notfound") {
		t.Error("map shouldn't have this key")
	}

	if !MapContainsKey(m, "key1") {
		t.Error("map should have this key")
	}
}

func TestMapIntKeysToSortedSlice(t *testing.T) {
	m := map[int]string{
		15:  "fifteen",
		20:  "twenty",
		0:   "zero",
		-10: "negative ten",
	}

	expected := []string{
		"negative ten",
		"zero",
		"fifteen",
		"twenty",
	}

	actual := MapIntKeysToSortedSlice(m)

	t.Logf("result %v", actual)

	if len(actual) != len(expected) {
		t.Errorf("unexpected result length %d", len(actual))
	}

	for i, v := range expected {
		if actual[i] != v {
			t.Errorf("expected %s, found %s", v, actual[i])
		}
	}
}
