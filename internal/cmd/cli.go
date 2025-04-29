package cmd

import (
	"context"
	"slices"

	"github.com/MetroStar/quartzctl/internal/log"
	"github.com/MetroStar/quartzctl/internal/util"
	"github.com/urfave/cli/v3"
	"go.uber.org/fx"
)

// CliDependencies contains the dependencies required to initialize the CLI.
// It is used to inject parameters, root commands, and build metadata.
//
// Fields:
//   - Params: CommandParams for managing CLI configuration and secrets.
//   - Root: RootCommandParams containing the list of root-level commands.
//   - Version: The version of the CLI.
//   - BuildDate: The build date of the CLI.
type CliDependencies struct {
	fx.In
	Params *CommandParams
	Root   RootCommandParams
}

// NewCliCommand creates a new CLI command with the provided dependencies.
//
// Parameters:
//   - deps: CliDependencies containing the required dependencies.
//
// Returns:
//   - *cli.Command: The root CLI command for the Quartz tool.
func NewCliCommand(deps CliDependencies, p AppServiceParams) *cli.Command {
	cli.VersionPrinter = func(ccmd *cli.Command) {
		configureLogger(ccmd)
		Version(p.Version, p.BuildDate)
	}

	slices.SortFunc(deps.Root.Commands, ByCommandName)

	return &cli.Command{
		Version:               p.Version,
		Name:                  "quartz",
		Description:           "Quartz cloud/kubernetes platform automation tool",
		Usage:                 "\b\b ",
		EnableShellCompletion: true,
		Commands:              deps.Root.Commands,
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "config", Aliases: []string{"c"}, Usage: "override default config file", Value: "./quartz.yaml"},
			&cli.StringFlag{Name: "secrets", Usage: "configure secrets with yaml"},
		},
		// Before is executed before the command runs to set up configuration and secrets.
		Before: func(ctx context.Context, ccmd *cli.Command) (context.Context, error) {
			configureLogger(ccmd)
			deps.Params.SetConfig(ccmd.String("config"))
			deps.Params.SetSecrets(ccmd.String("secrets"))
			return ctx, nil
		},
	}
}

// configureLogger sets up the logger for the CLI command.
//
// Parameters:
//   - ccmd: The CLI command for which the logger is being configured.
//
// This function configures the logger to use the output writer of the root command
// and applies the configuration file specified by the "config" flag.
func configureLogger(ccmd *cli.Command) {
	w := ccmd.Root().Writer
	util.SetWriter(w)
	log.ConfigureDefault(ccmd.String("config"), w)
}
