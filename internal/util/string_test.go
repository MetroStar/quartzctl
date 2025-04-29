package util

import "testing"

func TestTruncateStringHappy(t *testing.T) {
	s := "this is my test string"
	n := 7

	expected := "this is"
	actual := TruncateString(s, n)
	if actual != expected {
		t.Errorf("expected %s, got %s", expected, actual)
	}
}

func TestTruncateStringNoop(t *testing.T) {
	s := "this is my test string"
	n := len(s) + 1

	expected := s
	actual := TruncateString(s, n)
	if actual != expected {
		t.Errorf("expected %s, got %s", expected, actual)
	}
}

func TestTruncateStringEHappy(t *testing.T) {
	s := "this is my test string"
	n := 7

	expected := "this..."
	actual := TruncateStringE(s, n)
	if actual != expected {
		t.Errorf("expected %s, got %s", expected, actual)
	}
}

func TestTruncateStringENoop(t *testing.T) {
	s := "this is my test string"
	n := len(s) + 1

	expected := s
	actual := TruncateStringE(s, n)
	if actual != expected {
		t.Errorf("expected %s, got %s", expected, actual)
	}
}

func TestTruncateStringETooShort(t *testing.T) {
	s := "this is my test string"
	n := 2

	expected := "th"
	actual := TruncateStringE(s, n)
	if actual != expected {
		t.Errorf("expected %s, got %s", expected, actual)
	}
}

func TestUtilEqualsOrRegexMatchString(t *testing.T) {
	type test struct {
		pattern     string
		val         string
		insensitive bool
		expected    bool
		message     string
	}

	tests := []test{
		// case sensitive
		{"foobar", "foobar", false, true, "failed to match equal string literals"},
		{"foobar", "foobar1", false, true, "failed to match substring"},
		{"foo", "bar", false, false, "incorrectly matched unequal strings"},
		{"foobar", "FooBar", false, false, "incorrectly matched mixed case strings"},
		{"^foo", "foobar", false, true, "failed to match starts with regex pattern"},
		{"^foo", "123foobar", false, false, "incorrectly matched regex pattern"},

		// case insensitive
		{"foobar", "FooBar", true, true, "failed to match mixed case strings (case insensitive)"},
		{"foobar", "FooBar1", true, true, "failed to match substring (case insensitive)"},
		{"^foo", "FooBar", true, true, "failed to match starts with regex pattern (case insensitive)"},
		{"^foo", "123foobar", true, false, "incorrectly matched regex pattern (case insensitive)"},
	}

	for _, tc := range tests {
		if tc.expected && !EqualsOrRegexMatchString(tc.pattern, tc.val, tc.insensitive) {
			t.Error(tc.message)
		}

		if !tc.expected && EqualsOrRegexMatchString(tc.pattern, tc.val, tc.insensitive) {
			t.Error(tc.message)
		}
	}
}
