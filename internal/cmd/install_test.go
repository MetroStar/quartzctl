package cmd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"
)

func TestNewRootInstallCommand(t *testing.T) {
	p := defaultTestConfig(t)
	cmd := NewRootInstallCommand(p).Command

	assert.Equal(t, "install", cmd.Name)
	assert.Equal(t, "Perform a full install/update of the system", cmd.Usage)

	err := cmd.Action(context.Background(), &cli.Command{})
	assert.NoError(t, err)
}

func TestNewRootCleanCommand(t *testing.T) {
	p := defaultTestConfig(t)
	cmd := NewRootCleanCommand(p).Command

	assert.Equal(t, "clean", cmd.Name)
	assert.Equal(t, "Perform a full cleanup/teardown of the system", cmd.Usage)
	assert.Len(t, cmd.Flags, 1)

	flag := cmd.Flags[0].(*cli.BoolFlag)
	assert.Equal(t, "refresh", flag.Name)

	err := cmd.Action(context.Background(), &cli.Command{})
	assert.NoError(t, err)
}

func TestCmdInstall(t *testing.T) {
	p := defaultTestConfig(t)

	err := Install(context.Background(), p)
	if err != nil {
		t.Errorf("unexpected error in cmd Install, %v", err)
	}
}

func TestCmdClean(t *testing.T) {
	p := defaultTestConfig(t)

	err := Clean(context.Background(), true, p)
	if err != nil {
		t.Errorf("unexpected error in cmd Clean, %v", err)
	}
}
