package util

import "testing"

func TestRunOnce(t *testing.T) {
	count := 0
	countFunc := func() error {
		count = count + 1
		return nil
	}

	// first call, increment count from 0 -> 1
	err := RunOnce("test", countFunc)
	if err != nil {
		t.Errorf("unexpected error in RunOnce, %v", err)
	}

	if count != 1 {
		t.Errorf("unexpected run count in RunOnce first call, expected 1, found %d", count)
	}

	// second call with the same key, no action, count remains 1
	RunOnce("test", countFunc)

	if count != 1 {
		t.Errorf("unexpected run count in RunOnce second call, expected 1, found %d", count)
	}

	// third call with different key, increment count from 1 -> 2
	RunOnce("different", countFunc)

	if count != 2 {
		t.Errorf("unexpected run count in RunOnce third call, expected 2, found %d", count)
	}
}
