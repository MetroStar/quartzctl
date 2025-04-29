package cmd

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"
)

const (
	testStage = "first"
)

func TestNewTfInitCommand(t *testing.T) {
	p := defaultTestConfig(t)
	cmd := NewTfInitCommand(p).Command

	assert.Equal(t, "init", cmd.Name)
	assert.Equal(t, "Run `terraform init` for a specific stage", cmd.Usage)
	assert.Len(t, cmd.Flags, 1)

	flag := cmd.Flags[0].(*cli.StringFlag)
	assert.Equal(t, "stage", flag.Name)
	assert.True(t, flag.Required)

	runTestTfCommandWithStage(t, cmd)
}

func TestNewTfInitAllCommand(t *testing.T) {
	p := defaultTestConfig(t)
	cmd := NewTfInitAllCommand(p).Command

	assert.Equal(t, "init-all", cmd.Name)
	assert.Equal(t, "Run `terraform init` for all stages", cmd.Usage)

	runTestTfCommand(t, cmd)
}

func TestNewTfApplyCommand(t *testing.T) {
	p := defaultTestConfig(t)
	cmd := NewTfApplyCommand(p).Command

	assert.Equal(t, "apply", cmd.Name)
	assert.Equal(t, "Run `terraform apply` for a specific stage", cmd.Usage)
	assert.Len(t, cmd.Flags, 2)

	stageFlag := cmd.Flags[0].(*cli.StringFlag)
	assert.Equal(t, "stage", stageFlag.Name)
	assert.True(t, stageFlag.Required)

	initFlag := cmd.Flags[1].(*cli.BoolFlag)
	assert.Equal(t, "init", initFlag.Name)

	runTestTfCommandWithStage(t, cmd)
}

func TestNewTfPlanCommand(t *testing.T) {
	p := defaultTestConfig(t)
	cmd := NewTfPlanCommand(p).Command

	assert.Equal(t, "plan", cmd.Name)
	assert.Equal(t, "Run `terraform plan` for a specific stage", cmd.Usage)
	assert.Len(t, cmd.Flags, 2)

	stageFlag := cmd.Flags[0].(*cli.StringFlag)
	assert.Equal(t, "stage", stageFlag.Name)
	assert.True(t, stageFlag.Required)

	initFlag := cmd.Flags[1].(*cli.BoolFlag)
	assert.Equal(t, "init", initFlag.Name)

	runTestTfCommandWithStage(t, cmd)
}

func TestNewTfDestroyCommand(t *testing.T) {
	p := defaultTestConfig(t)
	cmd := NewTfDestroyCommand(p).Command

	assert.Equal(t, "destroy", cmd.Name)
	assert.Equal(t, "Run `terraform destroy` for a specific stage", cmd.Usage)
	assert.Len(t, cmd.Flags, 2)

	stageFlag := cmd.Flags[0].(*cli.StringFlag)
	assert.Equal(t, "stage", stageFlag.Name)
	assert.True(t, stageFlag.Required)

	initFlag := cmd.Flags[1].(*cli.BoolFlag)
	assert.Equal(t, "init", initFlag.Name)

	runTestTfCommandWithStage(t, cmd)
}

func TestNewTfOutputCommand(t *testing.T) {
	p := defaultTestConfig(t)
	cmd := NewTfOutputCommand(p).Command

	assert.Equal(t, "output", cmd.Name)
	assert.Equal(t, "Retrieve Terraform output for a specific stage", cmd.Usage)
	assert.Len(t, cmd.Flags, 2)

	stageFlag := cmd.Flags[0].(*cli.StringFlag)
	assert.Equal(t, "stage", stageFlag.Name)
	assert.True(t, stageFlag.Required)

	initFlag := cmd.Flags[1].(*cli.BoolFlag)
	assert.Equal(t, "init", initFlag.Name)

	runTestTfCommandWithStage(t, cmd)
}

func TestNewTfRefreshCommand(t *testing.T) {
	p := defaultTestConfig(t)
	cmd := NewTfRefreshCommand(p).Command

	assert.Equal(t, "refresh", cmd.Name)
	assert.Equal(t, "Run `terraform refresh` for a specific stage", cmd.Usage)
	assert.Len(t, cmd.Flags, 2)

	stageFlag := cmd.Flags[0].(*cli.StringFlag)
	assert.Equal(t, "stage", stageFlag.Name)
	assert.True(t, stageFlag.Required)

	initFlag := cmd.Flags[1].(*cli.BoolFlag)
	assert.Equal(t, "init", initFlag.Name)

	runTestTfCommandWithStage(t, cmd)
}

func TestNewTfRefreshAllCommand(t *testing.T) {
	p := defaultTestConfig(t)
	cmd := NewTfRefreshAllCommand(p).Command

	assert.Equal(t, "refresh-all", cmd.Name)
	assert.Equal(t, "Run `terraform refresh` for all stages", cmd.Usage)
	assert.Len(t, cmd.Flags, 1)

	initFlag := cmd.Flags[0].(*cli.BoolFlag)
	assert.Equal(t, "init", initFlag.Name)

	runTestTfCommand(t, cmd)
}

func TestNewTfValidateCommand(t *testing.T) {
	p := defaultTestConfig(t)
	cmd := NewTfValidateCommand(p).Command

	assert.Equal(t, "validate", cmd.Name)
	assert.Equal(t, "Run `terraform validate` for a specific stage", cmd.Usage)
	assert.Len(t, cmd.Flags, 1)

	stageFlag := cmd.Flags[0].(*cli.StringFlag)
	assert.Equal(t, "stage", stageFlag.Name)
	assert.True(t, stageFlag.Required)

	runTestTfCommandWithStage(t, cmd)
}

func TestNewTfFormatCommand(t *testing.T) {
	p := defaultTestConfig(t)
	cmd := NewTfFormatCommand(p).Command

	assert.Equal(t, "format", cmd.Name)
	assert.Equal(t, "Run `terraform fmt` for a specific stage", cmd.Usage)
	assert.Len(t, cmd.Flags, 1)

	stageFlag := cmd.Flags[0].(*cli.StringFlag)
	assert.Equal(t, "stage", stageFlag.Name)
	assert.True(t, stageFlag.Required)

	runTestTfCommandWithStage(t, cmd)
}

func TestNewTfFormatAllCommand(t *testing.T) {
	p := defaultTestConfig(t)
	cmd := NewTfFormatAllCommand(p).Command

	assert.Equal(t, "format-all", cmd.Name)
	assert.Equal(t, "Run `terraform fmt` for all stages", cmd.Usage)

	runTestTfCommand(t, cmd)
}

func TestNewTfVersionCommand(t *testing.T) {
	p := defaultTestConfig(t)
	cmd := NewTfVersionCommand(p).Command

	assert.Equal(t, "version", cmd.Name)
	assert.Equal(t, "Check and display the Terraform version", cmd.Usage)

	runTestTfCommand(t, cmd)
}

func TestNewRootTerraformCommand(t *testing.T) {
	p := defaultTestConfig(t)
	cmds := TfCommandParams{
		Commands: []*cli.Command{
			{Name: "apply"},
			{Name: "plan"},
		},
	}
	cmd := NewRootTerraformCommand(cmds, p).Command

	assert.Equal(t, "terraform", cmd.Name)
	assert.Equal(t, "Terraform subcommands for individual stages", cmd.Usage)
	assert.Len(t, cmd.Commands, 2)
	assert.Equal(t, "apply", cmd.Commands[0].Name)
	assert.Equal(t, "plan", cmd.Commands[1].Name)
}

func TestCmdTfInit(t *testing.T) {
	p := defaultTestConfig(t)

	err := TfInit(context.Background(), testStage, p)
	if err != nil {
		t.Errorf("unexpected error in cmd TfInit, %v", err)
	}
}

func TestCmdTfInitAll(t *testing.T) {
	p := defaultTestConfig(t)

	err := TfInitAll(context.Background(), p)
	if err != nil {
		t.Errorf("unexpected error in cmd TfInitAll, %v", err)
	}
}

func TestCmdTfPlan(t *testing.T) {
	p := defaultTestConfig(t)

	TfInit(context.Background(), testStage, p)
	err := TfPlan(context.Background(), testStage, p)
	if err != nil {
		t.Errorf("unexpected error in cmd TfPlan, %v", err)
	}
}

func TestCmdTfApply(t *testing.T) {
	p := defaultTestConfig(t)

	TfInit(context.Background(), testStage, p)
	err := TfApply(context.Background(), testStage, p)
	if err != nil {
		t.Errorf("unexpected error in cmd TfApply, %v", err)
	}
}

func TestCmdTfDestroy(t *testing.T) {
	p := defaultTestConfig(t)

	TfInit(context.Background(), testStage, p)
	err := TfDestroy(context.Background(), testStage, p)
	if err != nil {
		t.Errorf("unexpected error in cmd TfDestroy, %v", err)
	}
}

func TestCmdTfOutput(t *testing.T) {
	p := defaultTestConfig(t)

	TfInit(context.Background(), testStage, p)
	err := TfOutput(context.Background(), testStage, p)
	if err != nil {
		t.Errorf("unexpected error in cmd TfOutput, %v", err)
	}
}

func TestCmdTfRefresh(t *testing.T) {
	p := defaultTestConfig(t)

	TfInit(context.Background(), testStage, p)
	err := TfRefresh(context.Background(), testStage, p)
	if err != nil {
		t.Errorf("unexpected error in cmd TfRefresh, %v", err)
	}
}

func TestCmdTfRefreshAll(t *testing.T) {
	p := defaultTestConfig(t)

	TfInit(context.Background(), testStage, p)
	err := TfRefreshAll(context.Background(), p)
	if err != nil {
		t.Errorf("unexpected error in cmd TfRefreshAll, %v", err)
	}
}

func TestCmdTfValidate(t *testing.T) {
	p := defaultTestConfig(t)

	_, err := TfValidate(context.Background(), testStage, p)
	if err != nil {
		t.Errorf("unexpected error in cmd TfValidate, %v", err)
	}
}

func TestCmdTfFormat(t *testing.T) {
	p := defaultTestConfig(t)

	err := TfFormat(context.Background(), testStage, p)
	if err != nil {
		t.Errorf("unexpected error in cmd TfFormat, %v", err)
	}
}

func TestCmdTfFormatAll(t *testing.T) {
	p := defaultTestConfig(t)

	err := TfFormatAll(context.Background(), p)
	if err != nil {
		t.Errorf("unexpected error in cmd TfFormatAll, %v", err)
	}
}

func TestCmdTfVersion(t *testing.T) {
	p := defaultTestConfig(t)

	err := TfVersion(context.Background(), p)
	if err != nil {
		t.Errorf("unexpected error in cmd TfVersion, %v", err)
	}
}

func TestCmdTfCreateBackend(t *testing.T) {
	p := defaultTestConfig(t)

	err := TfCreateBackend(context.Background(), p)
	if err != nil {
		t.Errorf("unexpected error in cmd TfCreateBackend, %v", err)
	}
}

func TestCmdTfDestroyBackend(t *testing.T) {
	p := defaultTestConfig(t)

	err := TfDestroyBackend(context.Background(), p)
	if err != nil {
		t.Errorf("unexpected error in cmd TfDestroyBackend, %v", err)
	}
}

func runTestTfCommand(t *testing.T, cmd *cli.Command, args ...string) {
	err := cmd.Run(context.Background(), append([]string{cmd.Name}, args...))
	assert.NoError(t, err)
}

func runTestTfCommandWithStage(t *testing.T, cmd *cli.Command) {
	err := cmd.Run(context.Background(), []string{cmd.Name})
	assert.Error(t, err) // Missing required flag

	runTestTfCommand(t, cmd, "-s", "first")
}
