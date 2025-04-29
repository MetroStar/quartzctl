package cmd

import (
	"context"
	"fmt"

	"github.com/MetroStar/quartzctl/internal/log"
	"github.com/MetroStar/quartzctl/internal/util"
	"github.com/urfave/cli/v3"
)

// NewRootInternalCommand creates the "internal" root command for the CLI.
// This command is hidden and is used for internal operations such as force cleanup.
//
// Parameters:
//   - p: *CommandParams containing configuration and runtime parameters.
//
// Returns:
//   - RootCommandResult containing the "internal" CLI command.
func NewRootInternalCommand(p *CommandParams) RootCommandResult {
	return RootCommandResult{
		Command: &cli.Command{
			Name:   "internal",
			Hidden: true,
			Commands: []*cli.Command{
				{
					Name:  "force-cleanup",
					Usage: "Perform post-delete cleanup actions",
					Action: func(ctx context.Context, ccmd *cli.Command) error {
						return ForceCleanup(ctx, p)
					},
				},
			},
		},
	}
}

// ForceCleanup performs post-delete cleanup, including removing temporary files
// and destroying the Terraform state bucket.
//
// Parameters:
//   - ctx: The context for the operation.
//   - p: *CommandParams containing configuration and runtime parameters.
//
// Returns:
//   - error: An error if the cleanup fails, otherwise nil.
func ForceCleanup(ctx context.Context, p *CommandParams) error {
	log.Debug("Entering", "command", "internal:forceCleanup")
	defer log.Debug("Completed", "command", "internal:forceCleanup")

	util.Errorf("Manually executing post delete cleanup actions")
	if r := util.PromptYesNo("This cannot be undone, are you sure?"); !r {
		return fmt.Errorf("aborting")
	}

	// Destroy the Terraform backend
	err := TfDestroyBackend(ctx, p)
	if err != nil {
		return err
	}

	// Perform additional cleanup actions
	err = Cleanup(ctx, p)
	if err != nil {
		return err
	}

	return nil
}
