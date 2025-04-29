package cmd

import (
	"context"
	"testing"
)

func TestCmdForceCleanup(t *testing.T) {
	p := defaultTestConfig(t)

	err := ForceCleanup(context.Background(), p)
	if err != nil {
		t.Errorf("unexpected error in cmd ForceCleanup, %v", err)
	}
}
