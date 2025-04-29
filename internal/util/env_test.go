package util

import "testing"

func TestUtilGetenvOrDefault(t *testing.T) {
	key := "MY_TEST_VAR"

	// ensure it's unset to begin with, should get the default value back
	t.Setenv(key, "")
	actual1 := GetenvOrDefault(key, "foobar")
	if actual1 != "foobar" {
		t.Errorf("incorrect value for env var %s, expected %s, found %s", key, "foobar", actual1)
	}

	// give it a value to test the "has value" path
	t.Setenv(key, "something")
	actual2 := GetenvOrDefault(key, "foobar")
	if actual2 != "something" {
		t.Errorf("incorrect value for env var %s, expected %s, found %s", key, "something", actual2)
	}
}

func TestUtilOsEnvMap(t *testing.T) {
	t.Setenv("TEST_VAR_1", "foo")
	t.Setenv("TEST_VAR_2", "bar")

	actual := OsEnvMap()

	expected := map[string]string{
		"TEST_VAR_1": "foo",
		"TEST_VAR_2": "bar",
	}

	for k, v := range expected {
		a, ok := actual[k]
		if !ok {
			t.Errorf("key not found, %s", k)
		} else if a != v {
			t.Errorf("incorrect value extracted from env map, %s, expected %s, found %s", k, v, a)
		}
	}
}

func TestUtilEnvMap(t *testing.T) {
	input := []string{
		"TEST_VAR_1=foo",
		"TEST_VAR_2=bar",
	}

	actual := EnvMap(input)

	expected := map[string]string{
		"TEST_VAR_1": "foo",
		"TEST_VAR_2": "bar",
	}

	if len(actual) != len(expected) {
		t.Errorf("mismatched lengths, expected %d, found %d", len(expected), len(actual))
	}

	for k, v := range expected {
		a, ok := actual[k]
		if !ok {
			t.Errorf("key not found, %s", k)
		} else if a != v {
			t.Errorf("incorrect value extracted from env map, %s, expected %s, found %s", k, v, a)
		}
	}
}
