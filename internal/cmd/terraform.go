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

package cmd

import (
	"context"
	"slices"

	"github.com/MetroStar/quartzctl/internal/log"
	"github.com/MetroStar/quartzctl/internal/stages"
	"github.com/MetroStar/quartzctl/internal/terraform"
	"github.com/MetroStar/quartzctl/internal/util"
	"github.com/urfave/cli/v3"
)

// NewRootTerraformCommand creates the "terraform" root command for the CLI.
// This command provides subcommands for managing Terraform stages.
//
// Parameters:
//   - cmds: TfCommandParams containing the list of Terraform subcommands.
//   - p: *CommandParams containing configuration and runtime parameters.
//
// Returns:
//   - RootCommandResult containing the "terraform" CLI command.
func NewRootTerraformCommand(cmds TfCommandParams, p *CommandParams) RootCommandResult {
	slices.SortFunc(cmds.Commands, ByCommandName)
	return RootCommandResult{
		Command: &cli.Command{
			Name:     "terraform",
			Aliases:  []string{"tf"},
			Usage:    "Terraform subcommands for individual stages",
			Commands: cmds.Commands,
		},
	}
}

// NewTfInitCommand creates a CLI command for running `terraform init` on a specific stage.
//
// Parameters:
//   - p: *CommandParams containing configuration and runtime parameters.
//
// Returns:
//   - TfCommandResult containing the "init" CLI command.
func NewTfInitCommand(p *CommandParams) TfCommandResult {
	return TfCommandResult{
		Command: &cli.Command{
			Name:  "init",
			Usage: "Run `terraform init` for a specific stage",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "stage", Aliases: []string{"s"}, Usage: "Stage name", Required: true},
			},
			Action: func(ctx context.Context, ccmd *cli.Command) error {
				stage := ccmd.String("stage")
				return TfInit(ctx, stage, p)
			},
		},
	}
}

// NewTfInitAllCommand creates a CLI command for running `terraform init` on all stages.
//
// Parameters:
//   - p: *CommandParams containing configuration and runtime parameters.
//
// Returns:
//   - TfCommandResult containing the "init-all" CLI command.
func NewTfInitAllCommand(p *CommandParams) TfCommandResult {
	return TfCommandResult{
		Command: &cli.Command{
			Name:  "init-all",
			Usage: "Run `terraform init` for all stages",
			Action: func(ctx context.Context, ccmd *cli.Command) error {
				return TfInitAll(ctx, p)
			},
		},
	}
}

// NewTfApplyCommand creates a CLI command for running `terraform apply` on a specific stage.
//
// Parameters:
//   - p: *CommandParams containing configuration and runtime parameters.
//
// Returns:
//   - TfCommandResult containing the "apply" CLI command.
func NewTfApplyCommand(p *CommandParams) TfCommandResult {
	return TfCommandResult{
		Command: &cli.Command{
			Name:  "apply",
			Usage: "Run `terraform apply` for a specific stage",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "stage", Aliases: []string{"s"}, Usage: "Stage name", Required: true},
				&cli.BoolFlag{Name: "init", Aliases: []string{"i"}, Usage: "Run `terraform init` before applying", Required: false},
			},
			Action: func(ctx context.Context, ccmd *cli.Command) error {
				stage := ccmd.String("stage")
				init := ccmd.Bool("init")
				if init {
					err := TfInit(ctx, stage, p)
					if err != nil {
						return err
					}
				}
				return TfApply(ctx, stage, p)
			},
		},
	}
}

// NewTfPlanCommand creates a CLI command for running `terraform plan` on a specific stage.
//
// Parameters:
//   - p: *CommandParams containing configuration and runtime parameters.
//
// Returns:
//   - TfCommandResult containing the "plan" CLI command.
func NewTfPlanCommand(p *CommandParams) TfCommandResult {
	return TfCommandResult{
		Command: &cli.Command{
			Name:  "plan",
			Usage: "Run `terraform plan` for a specific stage",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "stage", Aliases: []string{"s"}, Usage: "Stage name", Required: true},
				&cli.BoolFlag{Name: "init", Aliases: []string{"i"}, Usage: "Run `terraform init` before planning", Required: false},
			},
			Action: func(ctx context.Context, ccmd *cli.Command) error {
				stage := ccmd.String("stage")
				init := ccmd.Bool("init")
				if init {
					err := TfInit(ctx, stage, p)
					if err != nil {
						return err
					}
				}
				return TfPlan(ctx, stage, p)
			},
		},
	}
}

// NewTfDestroyCommand creates a CLI command for running `terraform destroy` on a specific stage.
//
// Parameters:
//   - p: *CommandParams containing configuration and runtime parameters.
//
// Returns:
//   - TfCommandResult containing the "destroy" CLI command.
func NewTfDestroyCommand(p *CommandParams) TfCommandResult {
	return TfCommandResult{
		Command: &cli.Command{
			Name:  "destroy",
			Usage: "Run `terraform destroy` for a specific stage",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "stage", Aliases: []string{"s"}, Usage: "Stage name", Required: true},
				&cli.BoolFlag{Name: "init", Aliases: []string{"i"}, Usage: "Run `terraform init` before destroying", Required: false},
			},
			Action: func(ctx context.Context, ccmd *cli.Command) error {
				stage := ccmd.String("stage")
				init := ccmd.Bool("init")
				if init {
					err := TfInit(ctx, stage, p)
					if err != nil {
						return err
					}
				}
				return TfDestroy(ctx, stage, p)
			},
		},
	}
}

// NewTfOutputCommand creates a CLI command for retrieving Terraform output for a specific stage.
func NewTfOutputCommand(p *CommandParams) TfCommandResult {
	return TfCommandResult{
		Command: &cli.Command{
			Name:  "output",
			Usage: "Retrieve Terraform output for a specific stage",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "stage", Aliases: []string{"s"}, Usage: "Stage name", Required: true},
				&cli.BoolFlag{Name: "init", Aliases: []string{"i"}, Usage: "Run `terraform init` before retrieving output", Required: false},
			},
			Action: func(ctx context.Context, ccmd *cli.Command) error {
				stage := ccmd.String("stage")
				init := ccmd.Bool("init")
				if init {
					err := TfInit(ctx, stage, p)
					if err != nil {
						return err
					}
				}
				return TfOutput(ctx, stage, p)
			},
		},
	}
}

// NewTfRefreshCommand creates a CLI command for running `terraform refresh` on a specific stage.
func NewTfRefreshCommand(p *CommandParams) TfCommandResult {
	return TfCommandResult{
		Command: &cli.Command{
			Name:  "refresh",
			Usage: "Run `terraform refresh` for a specific stage",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "stage", Aliases: []string{"s"}, Usage: "Stage name", Required: true},
				&cli.BoolFlag{Name: "init", Aliases: []string{"i"}, Usage: "Run `terraform init` before refreshing", Required: false},
			},
			Action: func(ctx context.Context, ccmd *cli.Command) error {
				stage := ccmd.String("stage")
				init := ccmd.Bool("init")
				if init {
					err := TfInit(ctx, stage, p)
					if err != nil {
						return err
					}
				}
				return TfRefresh(ctx, stage, p)
			},
		},
	}
}

// NewTfRefreshAllCommand creates a CLI command for running `terraform refresh` on all stages.
func NewTfRefreshAllCommand(p *CommandParams) TfCommandResult {
	return TfCommandResult{
		Command: &cli.Command{
			Name:  "refresh-all",
			Usage: "Run `terraform refresh` for all stages",
			Flags: []cli.Flag{
				&cli.BoolFlag{Name: "init", Aliases: []string{"i"}, Usage: "Run `terraform init` before refreshing all stages", Required: false},
			},
			Action: func(ctx context.Context, ccmd *cli.Command) error {
				init := ccmd.Bool("init")
				if init {
					err := TfInitAll(ctx, p)
					if err != nil {
						return err
					}
				}
				return TfRefreshAll(ctx, p)
			},
		},
	}
}

// NewTfValidateCommand creates a CLI command for running `terraform validate` on a specific stage.
func NewTfValidateCommand(p *CommandParams) TfCommandResult {
	return TfCommandResult{
		Command: &cli.Command{
			Name:  "validate",
			Usage: "Run `terraform validate` for a specific stage",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "stage", Aliases: []string{"s"}, Usage: "Stage name", Required: true},
			},
			Action: func(ctx context.Context, ccmd *cli.Command) error {
				stage := ccmd.String("stage")
				_, err := TfValidate(ctx, stage, p)
				return err
			},
		},
	}
}

// NewTfFormatCommand creates a CLI command for running `terraform fmt` on a specific stage.
func NewTfFormatCommand(p *CommandParams) TfCommandResult {
	return TfCommandResult{
		Command: &cli.Command{
			Name:    "format",
			Usage:   "Run `terraform fmt` for a specific stage",
			Aliases: []string{"fmt"},
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "stage", Aliases: []string{"s"}, Usage: "Stage name", Required: true},
			},
			Action: func(ctx context.Context, ccmd *cli.Command) error {
				stage := ccmd.String("stage")
				return TfFormat(ctx, stage, p)
			},
		},
	}
}

// NewTfFormatAllCommand creates a CLI command for running `terraform fmt` on all stages.
func NewTfFormatAllCommand(p *CommandParams) TfCommandResult {
	return TfCommandResult{
		Command: &cli.Command{
			Name:  "format-all",
			Usage: "Run `terraform fmt` for all stages",
			Action: func(ctx context.Context, ccmd *cli.Command) error {
				return TfFormatAll(ctx, p)
			},
		},
	}
}

// NewTfVersionCommand creates a CLI command for checking and displaying the Terraform version.
func NewTfVersionCommand(p *CommandParams) TfCommandResult {
	return TfCommandResult{
		Command: &cli.Command{
			Name:  "version",
			Usage: "Check and display the Terraform version",
			Action: func(ctx context.Context, ccmd *cli.Command) error {
				return TfVersion(ctx, p)
			},
		},
	}
}

// TfInit runs `terraform init` for a specific stage.
func TfInit(ctx context.Context, stage string, p *CommandParams) error {
	return util.RunOnce("tf:init:"+stage, func() error {
		log.Debug("Entering", "command", "tf:init", "stage", stage)
		defer log.Debug("Completed", "command", "tf:init", "stage", stage)

		util.Hdrf("Init %s", stage)

		client := terraform.Instance(ctx, *p.Settings())
		err := tfStagePrep(ctx, stage, p)
		if err != nil {
			return err
		}

		cp, _ := p.Provider().Cloud(ctx)
		b := cp.StateBackendInfo(stage) // TODO, clean this up

		return wrapChecks(ctx, stage, "init", p, func() error {
			s := p.Settings().Config.Stages[stage]
			return client.Init(ctx, s, terraform.TerraformInitOpts{
				BackendConfig: b.InitBackendConfig,
			})
		})
	})
}

// TfInitAll runs `terraform init` for all stages.
func TfInitAll(ctx context.Context, p *CommandParams) error {
	log.Debug("Entering", "command", "tf:initAll")
	defer log.Debug("Completed", "command", "tf:initAll")

	err := tfStagePrep(ctx, "", p)
	if err != nil {
		return err
	}

	for _, s := range p.Settings().Config.StagesOrdered() {
		err = TfInit(ctx, s.Id, p)
		if err != nil {
			return err
		}
	}

	return nil
}

// TfPlan runs `terraform plan` for a specific stage.
func TfPlan(ctx context.Context, stage string, p *CommandParams) error {
	log.Debug("Entering", "command", "tf:plan", "stage", stage)
	defer log.Debug("Completed", "command", "tf:plan", "stage", stage)

	util.Hdrf("Plan %s", stage)

	client := terraform.Instance(ctx, *p.Settings())
	err := tfStagePrep(ctx, stage, p)
	if err != nil {
		return err
	}

	return wrapChecks(ctx, stage, "plan", p, func() error {
		s := p.Settings().Config.Stages[stage]
		empty, err := client.Plan(ctx, s)
		if !empty {
			log.Info("plan contains changes", "path", p.Settings().Config.Stages[stage].Path)
		}
		return err
	})
}

// TfApply runs `terraform apply` for a specific stage.
func TfApply(ctx context.Context, stage string, p *CommandParams) error {
	log.Debug("Entering", "command", "tf:apply", "stage", stage)
	defer log.Debug("Completed", "command", "tf:apply", "stage", stage)

	util.Hdrf("Apply %s", stage)

	client := terraform.Instance(ctx, *p.Settings())
	err := tfStagePrep(ctx, stage, p)
	if err != nil {
		return err
	}

	return wrapChecks(ctx, stage, "apply", p, func() error {
		s := p.Settings().Config.Stages[stage]
		return client.Apply(ctx, s)
	})
}

// TfDestroy runs `terraform destroy` for a specific stage.
func TfDestroy(ctx context.Context, stage string, p *CommandParams) error {
	log.Debug("Entering", "command", "tf:destroy", "stage", stage)
	defer log.Debug("Completed", "command", "tf:destroy", "stage", stage)

	util.Hdrf("Destroy %s", stage)

	client := terraform.Instance(ctx, *p.Settings())
	err := tfStagePrep(ctx, stage, p)
	if err != nil {
		return err
	}

	// can't run post checks after destroying the stage, just
	// checking prereqs instead
	err = preCheck(ctx, stage, "destroy", p)
	if err != nil {
		return err
	}

	s := p.Settings().Config.Stages[stage]
	return client.Destroy(ctx, s)
}

// TfOutput retrieves the Terraform output for a specific stage.
func TfOutput(ctx context.Context, stage string, p *CommandParams) error {
	return util.RunOnce("tf:output:"+stage, func() error {
		log.Debug("Entering", "command", "tf:output", "stage", stage)
		defer log.Debug("Completed", "command", "tf:output", "stage", stage)

		util.Hdrf("Output %s", stage)

		client := terraform.Instance(ctx, *p.Settings())
		s := p.Settings().Config.Stages[stage]
		err := tfStagePrep(ctx, stage, p)
		if err != nil {
			return err
		}

		o, err := client.Output(ctx, s)
		if err != nil {
			return err
		}

		for k, v := range o {
			util.Msgf("%s: %s", k, string(v))
		}

		return nil
	})
}

// TfRefresh runs `terraform refresh` for a specific stage.
func TfRefresh(ctx context.Context, stage string, p *CommandParams) error {
	return util.RunOnce("tf:refresh:"+stage, func() error {
		log.Debug("Entering", "command", "tf:refresh", "stage", stage)
		defer log.Debug("Completed", "command", "tf:refresh", "stage", stage)

		util.Hdrf("Refresh %s", stage)

		client := terraform.Instance(ctx, *p.Settings())
		err := tfStagePrep(ctx, stage, p)
		if err != nil {
			return err
		}

		s := p.Settings().Config.Stages[stage]
		err = client.Refresh(ctx, s)
		if err != nil {
			log.Info("Error refreshing terraform", "stage", s.Id, "err", err)
			return err
		}

		return nil
	})
}

// TfRefreshAll runs `terraform refresh` for all stages.
func TfRefreshAll(ctx context.Context, p *CommandParams) error {
	log.Debug("Entering", "command", "tf:refreshAll")
	defer log.Debug("Completed", "command", "tf:refreshAll")

	for _, s := range p.Settings().Config.StagesOrdered() {
		err := TfInit(ctx, s.Id, p)
		if err != nil {
			return err
		}

		err = TfRefresh(ctx, s.Id, p)
		if err != nil {
			return err
		}
	}

	return nil
}

// TfValidate runs `terraform validate` for a specific stage.
func TfValidate(ctx context.Context, stage string, p *CommandParams) (int, error) {
	log.Debug("Entering", "command", "tf:validate", "stage", stage)
	defer log.Debug("Completed", "command", "tf:validate", "stage", stage)

	util.Hdrf("Validate %s", stage)

	client := terraform.Instance(ctx, *p.Settings())
	s := p.Settings().Config.Stages[stage]
	v, err := client.Validate(ctx, s)
	return v.ErrorCount, err
}

// TfFormat runs `terraform fmt` for a specific stage.
func TfFormat(ctx context.Context, stage string, p *CommandParams) error {
	log.Debug("Entering", "command", "tf:format", "stage", stage)
	defer log.Debug("Completed", "command", "tf:format", "stage", stage)

	util.Hdrf("Format %s", stage)

	client := terraform.Instance(ctx, *p.Settings())
	s := p.Settings().Config.Stages[stage]
	return client.Format(ctx, s)
}

// TfFormatAll runs `terraform fmt` for all stages.
func TfFormatAll(ctx context.Context, p *CommandParams) error {
	log.Debug("Entering", "command", "tf:formatAll")
	defer log.Debug("Completed", "command", "tf:formatAll")

	for _, s := range p.Settings().Config.StagesOrdered() {
		err := TfFormat(ctx, s.Id, p)
		if err != nil {
			return err
		}
	}

	return nil
}

// TfVersion checks and displays the Terraform version.
func TfVersion(ctx context.Context, p *CommandParams) error {
	log.Debug("Entering", "command", "tf:version")
	defer log.Debug("Completed", "command", "tf:version")

	log.Debug("Querying Terraform client version...")

	client := terraform.Instance(ctx, *p.Settings())
	v, err := client.Version(ctx)
	if err != nil {
		return err
	}

	util.Msgf("Terraform version: %s\n", v)

	return nil
}

// TfCreateBackend creates the Terraform state backend.
func TfCreateBackend(ctx context.Context, p *CommandParams) error {
	log.Debug("Entering", "internal", "tf:createBackend")
	defer log.Debug("Completed", "internal", "tf:createBackend")

	util.Msg("Creating state backend")

	cp, _ := p.Provider().Cloud(ctx)
	return cp.CreateStateBackend(ctx)
}

// TfDestroyBackend destroys the Terraform state backend.
func TfDestroyBackend(ctx context.Context, p *CommandParams) error {
	log.Debug("Entering", "internal", "tf:DestroyBackend")
	defer log.Debug("Completed", "internal", "tf:DestroyBackend")

	util.Msg("Destroying state backend")

	cp, _ := p.Provider().Cloud(ctx)
	return cp.DestroyStateBackend(ctx)
}

// preCheck runs pre-checks for a specific stage and event.
func preCheck(ctx context.Context, stage string, event string, p *CommandParams) error {
	_, err := stages.RunPreChecks(ctx, p.Settings().Config, *p.Provider(), stage, event, checkOpts)
	return err
}

// postCheck runs post-checks for a specific stage and event.
func postCheck(ctx context.Context, stage string, event string, p *CommandParams) error {
	_, err := stages.RunPostChecks(ctx, p.Settings().Config, *p.Provider(), stage, event, checkOpts)
	return err
}

// wrapChecks wraps the execution of a function with pre-checks and post-checks.
func wrapChecks(ctx context.Context, stage string, event string, p *CommandParams, f func() error) error {
	err := preCheck(ctx, stage, event, p)
	if err != nil {
		return err
	}

	err = f()
	if err != nil {
		return err
	}

	return postCheck(ctx, stage, event, p)
}

// tfStagePrep prepares the Terraform stage for execution.
func tfStagePrep(ctx context.Context, stage string, p *CommandParams) error {
	err := util.RunOnce("tf:prep:0", func() error {
		return p.Settings().WriteJsonConfig(p.Settings().Config.TfVarFilePath(), "settings", true)
	})
	if err != nil {
		return err
	}

	if stage == "" {
		return nil
	}

	s := p.Settings().Config.Stages[stage]
	if !s.Providers.Kubernetes {
		return nil
	}

	return util.RunOnce("tf:prep:1", func() error {
		return ClusterLogin(ctx, "", p)
	})
}
