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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/MetroStar/quartzctl/internal/log"
	"github.com/MetroStar/quartzctl/internal/stages"
	"github.com/MetroStar/quartzctl/internal/tofu"
	"github.com/MetroStar/quartzctl/internal/util"
	"github.com/urfave/cli/v3"
)

// NewRootTofuCommand creates the "tofu" (aliased as "tf") root command for the CLI.
// This command provides subcommands for managing OpenTofu stages.
//
// Parameters:
//   - cmds: TfCommandParams containing the list of OpenTofu subcommands.
//   - p: *CommandParams containing configuration and runtime parameters.
//
// Returns:
//   - RootCommandResult containing the "tofu"/"tf" CLI command.
func NewRootTofuCommand(cmds TfCommandParams, p *CommandParams) RootCommandResult {
	slices.SortFunc(cmds.Commands, ByCommandName)
	return RootCommandResult{
		Command: &cli.Command{
			Name:     "tofu",
			Aliases:  []string{"tf"},
			Usage:    "OpenTofu subcommands for individual stages",
			Commands: cmds.Commands,
		},
	}
}

// NewTfInitCommand creates a CLI command for running `tofu init` on a specific stage.
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
			Usage: "Run `tofu init` for a specific stage",
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

// NewTfInitAllCommand creates a CLI command for running `tofu init` on all stages.
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
			Usage: "Run `tofu init` for all stages",
			Action: func(ctx context.Context, ccmd *cli.Command) error {
				return TfInitAll(ctx, p)
			},
		},
	}
}

// NewTfApplyCommand creates a CLI command for running `tofu apply` on a specific stage.
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
			Usage: "Run `tofu apply` for a specific stage",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "stage", Aliases: []string{"s"}, Usage: "Stage name", Required: true},
				&cli.BoolFlag{Name: "init", Aliases: []string{"i"}, Usage: "Run `tofu init` before applying", Required: false},
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

// NewTfPlanCommand creates a CLI command for running `tofu plan` on a specific stage.
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
			Usage: "Run `tofu plan` for a specific stage",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "stage", Aliases: []string{"s"}, Usage: "Stage name", Required: true},
				&cli.BoolFlag{Name: "init", Aliases: []string{"i"}, Usage: "Run `tofu init` before planning", Required: false},
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

// NewTfDestroyCommand creates a CLI command for running `tofu destroy` on a specific stage.
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
			Usage: "Run `tofu destroy` for a specific stage",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "stage", Aliases: []string{"s"}, Usage: "Stage name", Required: true},
				&cli.BoolFlag{Name: "init", Aliases: []string{"i"}, Usage: "Run `tofu init` before destroying", Required: false},
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

// NewTfOutputCommand creates a CLI command for retrieving OpenTofu output for a specific stage.
func NewTfOutputCommand(p *CommandParams) TfCommandResult {
	return TfCommandResult{
		Command: &cli.Command{
			Name:  "output",
			Usage: "Retrieve OpenTofu output for a specific stage",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "stage", Aliases: []string{"s"}, Usage: "Stage name", Required: true},
				&cli.BoolFlag{Name: "init", Aliases: []string{"i"}, Usage: "Run `tofu init` before retrieving output", Required: false},
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

// NewTfRefreshCommand creates a CLI command for running `tofu refresh` on a specific stage.
func NewTfRefreshCommand(p *CommandParams) TfCommandResult {
	return TfCommandResult{
		Command: &cli.Command{
			Name:  "refresh",
			Usage: "Run `tofu refresh` for a specific stage",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "stage", Aliases: []string{"s"}, Usage: "Stage name", Required: true},
				&cli.BoolFlag{Name: "init", Aliases: []string{"i"}, Usage: "Run `tofu init` before refreshing", Required: false},
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

// NewTfRefreshAllCommand creates a CLI command for running `tofu refresh` on all stages.
func NewTfRefreshAllCommand(p *CommandParams) TfCommandResult {
	return TfCommandResult{
		Command: &cli.Command{
			Name:  "refresh-all",
			Usage: "Run `tofu refresh` for all stages",
			Flags: []cli.Flag{
				&cli.BoolFlag{Name: "init", Aliases: []string{"i"}, Usage: "Run `tofu init` before refreshing all stages", Required: false},
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

// NewTfImportCommand creates a CLI command for running `tofu import` on a specific stage.
// This brings an existing infrastructure object under OpenTofu management, which is
// useful for reconciling state after an interrupted apply left a real resource created
// but unrecorded (e.g. an EKS OIDC association that succeeded just before the process died).
func NewTfImportCommand(p *CommandParams) TfCommandResult {
	return TfCommandResult{
		Command: &cli.Command{
			Name:      "import",
			Usage:     "Run `tofu import` for a specific stage",
			ArgsUsage: "ADDRESS ID",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "stage", Aliases: []string{"s"}, Usage: "Stage name", Required: true},
				&cli.StringFlag{Name: "address", Aliases: []string{"a"}, Usage: "Resource address to import into (e.g. aws_eks_identity_provider_config.keycloak)"},
				&cli.StringFlag{Name: "id", Usage: "Real-world ID of the resource to import"},
				&cli.BoolFlag{Name: "init", Aliases: []string{"i"}, Usage: "Run `tofu init` before importing", Required: false},
			},
			Action: func(ctx context.Context, ccmd *cli.Command) error {
				stage := ccmd.String("stage")

				// Accept the resource address and ID either as flags or as the
				// two positional args, matching the ergonomics of native `tofu import`.
				address := ccmd.String("address")
				id := ccmd.String("id")
				if address == "" && ccmd.Args().Len() > 0 {
					address = ccmd.Args().Get(0)
				}
				if id == "" && ccmd.Args().Len() > 1 {
					id = ccmd.Args().Get(1)
				}
				if address == "" || id == "" {
					return fmt.Errorf("import requires a resource address and ID (use --address/--id or positional ADDRESS ID)")
				}

				if ccmd.Bool("init") {
					if err := TfInit(ctx, stage, p); err != nil {
						return err
					}
				}
				return TfImport(ctx, stage, address, id, p)
			},
		},
	}
}

// NewTfForceUnlockCommand creates a CLI command for running `tofu force-unlock` on a specific stage.
// This releases a stale state lock left behind by an interrupted run so that subsequent
// operations can proceed without manually editing the backend lock table.
func NewTfForceUnlockCommand(p *CommandParams) TfCommandResult {
	return TfCommandResult{
		Command: &cli.Command{
			Name:      "force-unlock",
			Usage:     "Run `tofu force-unlock` for a specific stage",
			ArgsUsage: "LOCK_ID",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "stage", Aliases: []string{"s"}, Usage: "Stage name", Required: true},
				&cli.StringFlag{Name: "lock-id", Aliases: []string{"l"}, Usage: "Lock ID to release"},
				&cli.BoolFlag{Name: "init", Aliases: []string{"i"}, Usage: "Run `tofu init` before unlocking", Required: false},
			},
			Action: func(ctx context.Context, ccmd *cli.Command) error {
				stage := ccmd.String("stage")

				lockID := ccmd.String("lock-id")
				if lockID == "" && ccmd.Args().Len() > 0 {
					lockID = ccmd.Args().Get(0)
				}
				if lockID == "" {
					return fmt.Errorf("force-unlock requires a lock ID (use --lock-id or positional LOCK_ID)")
				}

				if ccmd.Bool("init") {
					if err := TfInit(ctx, stage, p); err != nil {
						return err
					}
				}
				return TfForceUnlock(ctx, stage, lockID, p)
			},
		},
	}
}

// NewTfValidateCommand creates a CLI command for running `tofu validate` on a specific stage.
func NewTfValidateCommand(p *CommandParams) TfCommandResult {
	return TfCommandResult{
		Command: &cli.Command{
			Name:  "validate",
			Usage: "Run `tofu validate` for a specific stage",
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

// NewTfFormatCommand creates a CLI command for running `tofu fmt` on a specific stage.
func NewTfFormatCommand(p *CommandParams) TfCommandResult {
	return TfCommandResult{
		Command: &cli.Command{
			Name:    "format",
			Usage:   "Run `tofu fmt` for a specific stage",
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

// NewTfFormatAllCommand creates a CLI command for running `tofu fmt` on all stages.
func NewTfFormatAllCommand(p *CommandParams) TfCommandResult {
	return TfCommandResult{
		Command: &cli.Command{
			Name:  "format-all",
			Usage: "Run `tofu fmt` for all stages",
			Action: func(ctx context.Context, ccmd *cli.Command) error {
				return TfFormatAll(ctx, p)
			},
		},
	}
}

// NewTfVersionCommand creates a CLI command for checking and displaying the OpenTofu version.
func NewTfVersionCommand(p *CommandParams) TfCommandResult {
	return TfCommandResult{
		Command: &cli.Command{
			Name:  "version",
			Usage: "Check and display the OpenTofu version",
			Action: func(ctx context.Context, ccmd *cli.Command) error {
				return TfVersion(ctx, p)
			},
		},
	}
}

// NewTfStateCommand creates the "state" subcommand group for inspecting and
// modifying the OpenTofu state of an individual stage. It mirrors the native
// `tofu state` workflow (list/show/rm) but routes through quartzctl so the
// correct backend, providers, and environment for a stage are resolved
// automatically. The `show` subcommand redacts sensitive attribute values so
// state can be inspected without leaking secrets.
func NewTfStateCommand(p *CommandParams) TfCommandResult {
	// Construct fresh flag instances per subcommand; sharing a single flag
	// pointer across commands is unsafe because cli stores parsed values on it.
	stageFlag := func() cli.Flag {
		return &cli.StringFlag{Name: "stage", Aliases: []string{"s"}, Usage: "Stage name", Required: true}
	}
	initFlag := func() cli.Flag {
		return &cli.BoolFlag{Name: "init", Aliases: []string{"i"}, Usage: "Run `tofu init` (backend only) before the state operation"}
	}

	return TfCommandResult{
		Command: &cli.Command{
			Name:  "state",
			Usage: "Inspect and modify OpenTofu state for a specific stage",
			Commands: []*cli.Command{
				{
					Name:      "list",
					Usage:     "List resource addresses in a stage's state",
					ArgsUsage: "[ADDRESS_FILTER...]",
					Flags:     []cli.Flag{stageFlag(), initFlag()},
					Action: func(ctx context.Context, ccmd *cli.Command) error {
						stage := ccmd.String("stage")
						if ccmd.Bool("init") {
							if err := TfStateInit(ctx, stage, p); err != nil {
								return err
							}
						}
						return TfStateList(ctx, stage, ccmd.Args().Slice(), p)
					},
				},
				{
					Name:      "show",
					Usage:     "Show resource attributes from a stage's state (sensitive values redacted)",
					ArgsUsage: "[ADDRESS...]",
					Flags:     []cli.Flag{stageFlag(), initFlag()},
					Action: func(ctx context.Context, ccmd *cli.Command) error {
						stage := ccmd.String("stage")
						if ccmd.Bool("init") {
							if err := TfStateInit(ctx, stage, p); err != nil {
								return err
							}
						}
						return TfStateShow(ctx, stage, ccmd.Args().Slice(), p)
					},
				},
				{
					Name:      "rm",
					Aliases:   []string{"remove"},
					Usage:     "Remove resources from a stage's state without destroying them",
					ArgsUsage: "ADDRESS [ADDRESS...]",
					Flags:     []cli.Flag{stageFlag(), initFlag()},
					Action: func(ctx context.Context, ccmd *cli.Command) error {
						stage := ccmd.String("stage")
						addresses := ccmd.Args().Slice()
						if len(addresses) == 0 {
							return fmt.Errorf("state rm requires at least one resource ADDRESS")
						}
						if ccmd.Bool("init") {
							if err := TfStateInit(ctx, stage, p); err != nil {
								return err
							}
						}
						return TfStateRemove(ctx, stage, addresses, p)
					},
				},
			},
		},
	}
}

// TfInit runs `tofu init` for a specific stage.
func TfInit(ctx context.Context, stage string, p *CommandParams) error {
	return util.RunOnce("tf:init:"+stage, func() error {
		log.Debug("Entering", "command", "tf:init", "stage", stage)
		defer log.Debug("Completed", "command", "tf:init", "stage", stage)

		util.Hdrf("Init %s", stage)

		client := tofu.Instance(ctx, *p.Settings())
		err := tfStagePrep(ctx, stage, p)
		if err != nil {
			return err
		}

		cp, _ := p.Provider().Cloud(ctx)
		b := cp.StateBackendInfo(stage) // TODO, clean this up

		return wrapChecks(ctx, stage, "init", p, func() error {
			s := p.Settings().Config.Stages[stage]
			return client.Init(ctx, s, tofu.TofuInitOpts{
				BackendConfig: b.InitBackendConfig,
			})
		})
	})
}

// TfInitAll runs `tofu init` for all stages.
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

// TfPlan runs `tofu plan` for a specific stage.
func TfPlan(ctx context.Context, stage string, p *CommandParams) error {
	log.Debug("Entering", "command", "tf:plan", "stage", stage)
	defer log.Debug("Completed", "command", "tf:plan", "stage", stage)

	util.Hdrf("Plan %s", stage)

	client := tofu.Instance(ctx, *p.Settings())
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

// TfApply runs `tofu apply` for a specific stage.
func TfApply(ctx context.Context, stage string, p *CommandParams) error {
	log.Debug("Entering", "command", "tf:apply", "stage", stage)
	defer log.Debug("Completed", "command", "tf:apply", "stage", stage)

	stageStart := time.Now()
	util.Hdrf("Apply %s", stage)

	client := tofu.Instance(ctx, *p.Settings())
	err := tfStagePrep(ctx, stage, p)
	if err != nil {
		return err
	}

	err = wrapChecks(ctx, stage, "apply", p, func() error {
		s := p.Settings().Config.Stages[stage]
		return client.Apply(ctx, s, tofu.TofuApplyOpts{AllowDeferral: p.allowDeferral})
	})

	util.Msgf("Stage %s completed in %v", stage, time.Since(stageStart).Round(time.Second))
	return err
}

// TfDestroy runs `tofu destroy` for a specific stage.
func TfDestroy(ctx context.Context, stage string, p *CommandParams) error {
	log.Debug("Entering", "command", "tf:destroy", "stage", stage)
	defer log.Debug("Completed", "command", "tf:destroy", "stage", stage)

	util.Hdrf("Destroy %s", stage)

	client := tofu.Instance(ctx, *p.Settings())
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

// TfOutput retrieves the OpenTofu output for a specific stage.
func TfOutput(ctx context.Context, stage string, p *CommandParams) error {
	return util.RunOnce("tf:output:"+stage, func() error {
		log.Debug("Entering", "command", "tf:output", "stage", stage)
		defer log.Debug("Completed", "command", "tf:output", "stage", stage)

		util.Hdrf("Output %s", stage)

		client := tofu.Instance(ctx, *p.Settings())
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

// TfRefresh runs `tofu refresh` for a specific stage.
func TfRefresh(ctx context.Context, stage string, p *CommandParams) error {
	return util.RunOnce("tf:refresh:"+stage, func() error {
		log.Debug("Entering", "command", "tf:refresh", "stage", stage)
		defer log.Debug("Completed", "command", "tf:refresh", "stage", stage)

		util.Hdrf("Refresh %s", stage)

		client := tofu.Instance(ctx, *p.Settings())
		err := tfStagePrep(ctx, stage, p)
		if err != nil {
			return err
		}

		s := p.Settings().Config.Stages[stage]
		err = client.Refresh(ctx, s)
		if err != nil {
			log.Info("Error refreshing tofu", "stage", s.Id, "err", err)
			return err
		}

		return nil
	})
}

// TfRefreshWithUnlock runs `tofu refresh` for a stage with automatic state lock recovery.
// If a stale lock is detected, it force-unlocks and retries once.
func TfRefreshWithUnlock(ctx context.Context, stage string, p *CommandParams) error {
	log.Debug("Entering", "command", "tf:refreshWithUnlock", "stage", stage)
	defer log.Debug("Completed", "command", "tf:refreshWithUnlock", "stage", stage)

	util.Hdrf("Refresh %s", stage)

	client := tofu.Instance(ctx, *p.Settings())
	err := tfStagePrep(ctx, stage, p)
	if err != nil {
		return err
	}

	s := p.Settings().Config.Stages[stage]
	err = client.Refresh(ctx, s)
	if err == nil {
		return nil
	}

	errStr := err.Error()
	if strings.Contains(errStr, "Error acquiring the state lock") || strings.Contains(errStr, "state blob is already locked") {
		if lockID, ok := tofu.ExtractLockID(errStr); ok {
			if unlockErr := client.ForceUnlock(ctx, s, lockID); unlockErr != nil {
				log.Warn("Force-unlock failed during refresh", "stage", stage, "lockID", lockID, "error", unlockErr)
			} else {
				util.Msgf("Successfully force-unlocked state for stage %s (lock ID: %s)", stage, lockID)
				// Retry refresh after unlock
				retryErr := client.Refresh(ctx, s)
				if retryErr == nil {
					return nil
				}
				log.Warn("Refresh still failed after force-unlock", "stage", stage, "error", retryErr)
				return retryErr
			}
		}
	}

	log.Info("Error refreshing tofu", "stage", s.Id, "err", err)
	return err
}

// TfImport runs `tofu import` for a specific stage, associating the resource at
// the given configuration address with its real-world ID.
func TfImport(ctx context.Context, stage string, address string, id string, p *CommandParams) error {
	log.Debug("Entering", "command", "tf:import", "stage", stage, "address", address, "id", id)
	defer log.Debug("Completed", "command", "tf:import", "stage", stage)

	util.Hdrf("Import %s", stage)

	client := tofu.Instance(ctx, *p.Settings())
	if err := tfStagePrep(ctx, stage, p); err != nil {
		return err
	}

	s := p.Settings().Config.Stages[stage]
	if err := client.Import(ctx, s, address, id); err != nil {
		return err
	}
	util.Msgf("Imported %s as %s into stage %s", id, address, stage)
	return nil
}

// TfForceUnlock runs `tofu force-unlock` for a specific stage, releasing a stale
// state lock left behind by an interrupted run.
func TfForceUnlock(ctx context.Context, stage string, lockID string, p *CommandParams) error {
	log.Debug("Entering", "command", "tf:forceUnlock", "stage", stage, "lockID", lockID)
	defer log.Debug("Completed", "command", "tf:forceUnlock", "stage", stage)

	util.Hdrf("Force-unlock %s", stage)

	client := tofu.Instance(ctx, *p.Settings())
	if err := tfStagePrep(ctx, stage, p); err != nil {
		return err
	}

	s := p.Settings().Config.Stages[stage]
	if err := client.ForceUnlock(ctx, s, lockID); err != nil {
		return err
	}
	util.Msgf("Released state lock %s for stage %s", lockID, stage)
	return nil
}

// TfStateInit runs a backend-only `tofu init` for a stage prior to a state
// operation. Unlike TfInit it deliberately skips the cluster-login step in
// tfStagePrep: state inspection/removal must work even when the stage's
// Kubernetes endpoint is gone (the exact situation where `state rm` is needed
// to drop orphaned resources).
func TfStateInit(ctx context.Context, stage string, p *CommandParams) error {
	log.Debug("Entering", "command", "tf:state:init", "stage", stage)
	defer log.Debug("Completed", "command", "tf:state:init", "stage", stage)

	util.Hdrf("Init %s", stage)

	client := tofu.Instance(ctx, *p.Settings())
	cp, _ := p.Provider().Cloud(ctx)
	b := cp.StateBackendInfo(stage)
	s := p.Settings().Config.Stages[stage]
	return client.Init(ctx, s, tofu.TofuInitOpts{
		BackendConfig: b.InitBackendConfig,
	})
}

// TfStateList prints the resource addresses recorded in a stage's state,
// optionally filtered by case-insensitive substring match.
func TfStateList(ctx context.Context, stage string, filters []string, p *CommandParams) error {
	log.Debug("Entering", "command", "tf:state:list", "stage", stage)
	defer log.Debug("Completed", "command", "tf:state:list", "stage", stage)

	util.Hdrf("State list %s", stage)

	client := tofu.Instance(ctx, *p.Settings())
	s := p.Settings().Config.Stages[stage]
	addresses, err := client.StateList(ctx, s, filters...)
	if err != nil {
		return err
	}

	if len(addresses) == 0 {
		util.Msgf("No resources in state for stage %s", stage)
		return nil
	}

	for _, addr := range addresses {
		util.Print(addr)
	}
	return nil
}

// TfStateShow prints the attributes of resources in a stage's state with
// sensitive values redacted. When no addresses are supplied every resource is
// shown.
func TfStateShow(ctx context.Context, stage string, addresses []string, p *CommandParams) error {
	log.Debug("Entering", "command", "tf:state:show", "stage", stage, "addresses", addresses)
	defer log.Debug("Completed", "command", "tf:state:show", "stage", stage)

	util.Hdrf("State show %s", stage)

	client := tofu.Instance(ctx, *p.Settings())
	s := p.Settings().Config.Stages[stage]
	views, err := client.StateShow(ctx, s, addresses...)
	if err != nil {
		return err
	}

	if len(views) == 0 {
		util.Msgf("No matching resources in state for stage %s", stage)
		return nil
	}

	for _, v := range views {
		j, err := json.MarshalIndent(v.Values, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to render resource %s: %w", v.Address, err)
		}
		util.Msgf("# %s", v.Address)
		util.Print(string(j))
	}
	return nil
}

// TfStateRemove removes the named resources from a stage's state without
// destroying the underlying infrastructure. It is primarily used to drop
// orphaned resources (e.g. a helm_release pointing at an already-deleted
// cluster) so a subsequent destroy/clean can proceed.
func TfStateRemove(ctx context.Context, stage string, addresses []string, p *CommandParams) error {
	log.Debug("Entering", "command", "tf:state:rm", "stage", stage, "addresses", addresses)
	defer log.Debug("Completed", "command", "tf:state:rm", "stage", stage)

	util.Hdrf("State rm %s", stage)

	client := tofu.Instance(ctx, *p.Settings())
	s := p.Settings().Config.Stages[stage]
	if err := client.StateRemove(ctx, s, addresses...); err != nil {
		return err
	}

	util.Msgf("Removed %d resource(s) from state for stage %s", len(addresses), stage)
	return nil
}

// TfRefreshAll runs `tofu refresh` for all stages.
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

// TfValidate runs `tofu validate` for a specific stage.
func TfValidate(ctx context.Context, stage string, p *CommandParams) (int, error) {
	log.Debug("Entering", "command", "tf:validate", "stage", stage)
	defer log.Debug("Completed", "command", "tf:validate", "stage", stage)

	util.Hdrf("Validate %s", stage)

	client := tofu.Instance(ctx, *p.Settings())
	s := p.Settings().Config.Stages[stage]
	v, err := client.Validate(ctx, s)
	if err != nil {
		return 0, err
	}
	// tofu writes the validate result to stdout for the user; the parsed
	// summary may be nil if the structured output could not be decoded
	// (e.g. version preamble on the stream). Guard against a nil deref.
	if v == nil {
		return 0, nil
	}
	return v.ErrorCount, nil
}

// TfFormat runs `tofu fmt` for a specific stage.
func TfFormat(ctx context.Context, stage string, p *CommandParams) error {
	log.Debug("Entering", "command", "tf:format", "stage", stage)
	defer log.Debug("Completed", "command", "tf:format", "stage", stage)

	util.Hdrf("Format %s", stage)

	client := tofu.Instance(ctx, *p.Settings())
	s := p.Settings().Config.Stages[stage]
	return client.Format(ctx, s)
}

// TfFormatAll runs `tofu fmt` for all stages.
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

// TfVersion checks and displays the OpenTofu version.
func TfVersion(ctx context.Context, p *CommandParams) error {
	log.Debug("Entering", "command", "tf:version")
	defer log.Debug("Completed", "command", "tf:version")

	log.Debug("Querying OpenTofu client version...")

	client := tofu.Instance(ctx, *p.Settings())
	v, err := client.Version(ctx)
	if err != nil {
		return err
	}

	util.Msgf("OpenTofu version: %s\n", v)

	return nil
}

// TfCreateBackend creates the OpenTofu state backend.
func TfCreateBackend(ctx context.Context, p *CommandParams) error {
	log.Debug("Entering", "internal", "tf:createBackend")
	defer log.Debug("Completed", "internal", "tf:createBackend")

	util.Msg("Creating state backend")

	cp, _ := p.Provider().Cloud(ctx)
	return cp.CreateStateBackend(ctx)
}

// TfDestroyBackend destroys the OpenTofu state backend.
func TfDestroyBackend(ctx context.Context, p *CommandParams) error {
	log.Debug("Entering", "internal", "tf:DestroyBackend")
	defer log.Debug("Completed", "internal", "tf:DestroyBackend")

	util.Msg("Destroying state backend")

	cp, _ := p.Provider().Cloud(ctx)
	return cp.DestroyStateBackend(ctx)
}

// TfStateBackendExists reports whether the OpenTofu state backend still exists.
// Used to detect an already-destroyed environment so clean can short-circuit.
func TfStateBackendExists(ctx context.Context, p *CommandParams) (bool, error) {
	log.Debug("Entering", "internal", "tf:stateBackendExists")
	defer log.Debug("Completed", "internal", "tf:stateBackendExists")

	cp, err := p.Provider().Cloud(ctx)
	if err != nil {
		return false, err
	}
	return cp.StateBackendExists(ctx)
}

// preCheck runs pre-checks for a specific stage and event.
//
// Stage pre-checks are the dependency gates that can block for many minutes
// waiting on prior-stage resources to converge (e.g. waiting on the istio
// HelmRelease and ingressgateway to become Ready before installing sonarqube).
// We run the same background safety nets here as in postCheck — the orphaned
// admission-webhook reaper (so a stranded failurePolicy: Fail webhook can't
// deadlock the very resources the gate is waiting on) and the periodic cluster
// progress reporter (so these long waits surface real readiness instead of
// looking hung).
func preCheck(ctx context.Context, stage string, event string, p *CommandParams) error {
	stopReaper := startWebhookReaper(ctx, p)
	defer stopReaper()

	stopProgress := startProgressReporter(ctx, p)
	defer stopProgress()

	_, err := stages.RunPreChecks(ctx, p.Settings().Config, *p.Provider(), stage, event, checkOpts)
	return err
}

// postCheck runs post-checks for a specific stage and event.
//
// While the post-checks run (and potentially retry for several minutes waiting
// on resources to become ready), a background safety net periodically reaps
// orphaned admission webhooks whose backing service has disappeared. This
// auto-heals the class of deadlock where an admission controller (e.g. Kyverno)
// is uninstalled mid-reconcile but its dynamically-created, failurePolicy: Fail
// webhook configurations are left behind, blocking ALL admission cluster-wide —
// which would otherwise hang a stage's post-install checks (e.g. waiting for the
// istio-cni-node daemonset) until the retry limit is exhausted.
func postCheck(ctx context.Context, stage string, event string, p *CommandParams) error {
	stopReaper := startWebhookReaper(ctx, p)
	defer stopReaper()

	stopProgress := startProgressReporter(ctx, p)
	defer stopProgress()

	_, err := stages.RunPostChecks(ctx, p.Settings().Config, *p.Provider(), stage, event, checkOpts)
	return err
}

// startWebhookReaper launches a background goroutine that periodically reaps
// orphaned admission webhooks (those whose backing service is gone) until the
// returned stop function is called. It is a no-op safety net when the cluster
// is not reachable. The returned function blocks until the goroutine exits.
func startWebhookReaper(ctx context.Context, p *CommandParams) func() {
	kube, err := p.Provider().Kubernetes(ctx)
	if err != nil {
		// Cluster not reachable (e.g. pre-cluster stages) — nothing to guard.
		return func() {}
	}

	reaperCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	go func() {
		defer close(done)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-reaperCtx.Done():
				return
			case <-ticker.C:
				reaped, rErr := kube.ReapOrphanedAdmissionWebhooks(reaperCtx)
				if rErr != nil {
					log.Debug("Webhook reaper safety net failed (non-fatal)", "err", rErr)
					continue
				}
				if len(reaped) > 0 {
					util.Msgf("Reaped %d orphaned admission webhook(s) blocking cluster admission: %v", len(reaped), reaped)
				}
			}
		}
	}()

	return func() {
		cancel()
		<-done
	}
}

// progressReportInterval returns the cadence for the install progress reporter.
// Defaults to 30s; override with QUARTZ_PROGRESS_INTERVAL (e.g. "15s", "1m").
func progressReportInterval() time.Duration {
	const def = 30 * time.Second
	if v := os.Getenv("QUARTZ_PROGRESS_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return def
}

// startProgressReporter launches a background goroutine that periodically logs a
// concise summary of cluster health (HelmRelease readiness, terminating
// namespaces, unhealthy pods) while a stage's post-install checks run. This
// gives users visibility into a deploying cluster instead of staring at a
// seemingly-idle install for minutes. It is a no-op when the cluster is not yet
// reachable (e.g. pre-cluster stages) or when disabled via QUARTZ_PROGRESS=off.
// The returned function blocks until the goroutine exits.
func startProgressReporter(ctx context.Context, p *CommandParams) func() {
	if strings.EqualFold(os.Getenv("QUARTZ_PROGRESS"), "off") {
		return func() {}
	}

	kube, err := p.Provider().Kubernetes(ctx)
	if err != nil {
		// Cluster not reachable yet — nothing to report.
		return func() {}
	}

	reportCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	go func() {
		defer close(done)
		interval := progressReportInterval()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		var lastSummary string
		report := func() {
			snap, sErr := kube.ClusterProgressSnapshot(reportCtx)
			// Skip emitting when the snapshot is unreliable: an explicit error,
			// or a canceled context (the reporter is being stopped, e.g. a stage
			// that skipped/completed in milliseconds). Emitting here would print
			// a misleading "0/0 ready" from a half-collected snapshot.
			if sErr != nil || reportCtx.Err() != nil {
				// A canceled context is the expected, benign case (the reporter is
				// being stopped). Only log genuine, unexpected snapshot errors;
				// logging cancellation is pure noise.
				if sErr != nil && !errors.Is(sErr, context.Canceled) {
					log.Debug("Progress reporter snapshot unavailable (non-fatal)", "err", sErr)
				}
				return
			}
			summary := snap.Summary()
			// Avoid spamming identical lines when nothing has changed.
			if summary == lastSummary {
				return
			}
			lastSummary = summary
			util.Msgf("Cluster progress: %s", summary)
			if len(snap.NotReadyReleases) > 0 {
				log.Debug("Not-ready HelmReleases", "releases", snap.NotReadyReleases)
			}
		}

		// Emit an initial reading promptly so users see status without waiting
		// a full interval.
		report()
		for {
			select {
			case <-reportCtx.Done():
				return
			case <-ticker.C:
				report()
			}
		}
	}()

	return func() {
		cancel()
		<-done
	}
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

// tfStagePrep prepares the OpenTofu stage for execution.
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
