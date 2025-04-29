package cmd

import (
	"context"
	"os"
	"time"

	"github.com/MetroStar/quartzctl/internal/log"
	"github.com/MetroStar/quartzctl/internal/util"
	"github.com/urfave/cli/v3"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
)

type AppServiceParams struct {
	Version   string
	BuildDate string
}

// RunAppService initializes and runs the application service using Uber's Fx framework.
// It sets up dependency injection, logging, and lifecycle hooks for the application.
func RunAppService(p AppServiceParams) {
	fx.New(
		fx.Supply(p),
		fx.Provide(
			NewAppService,
			NewCliCommand,
		),
		fx.WithLogger(func() fxevent.Logger {
			return log.NewFxLogger()
		}),
		RootCommandsModule,
		fx.Invoke(func(lc fx.Lifecycle, app *AppService) {
			// Append lifecycle hooks for starting and stopping the application.
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					return app.Start(os.Args)
				},
				OnStop: func(ctx context.Context) error {
					err := app.Stop()
					if err != nil {
						util.Printf("%v", err)
					}
					return nil
				},
			})
		}),
	).Run()
}

// RootCommandParams represents the input parameters for root-level commands.
// It is used to group root commands for dependency injection.
type RootCommandParams struct {
	fx.In
	Commands []*cli.Command `group:"root"`
}

// RootCommandResult represents the output result for a root-level command.
// It is used to group root commands for dependency injection.
type RootCommandResult struct {
	fx.Out
	Command *cli.Command `group:"root"`
}

// RootCommandsModule defines the root commands module for dependency injection.
// It provides all root-level commands and their dependencies.
var RootCommandsModule = fx.Module("rootCmds",
	fx.Provide(
		func() *CommandParams {
			return NewCommandParams(time.Now())
		},
		NewRootInstallCommand,
		NewRootCleanCommand,
		NewRootLoginCommand,
		NewRootInfoCommand,
		NewRootCheckCommand,
		NewRootRenderCommand,
		NewRootRefreshSecretsCommand,
		NewRootExportCommand,
		NewRootRestartCommand,
		NewRootTerraformCommand,
		NewRootAwsCommand,
		NewRootInternalCommand,
	),
	tfCommandsModule,
	awsCommandsModule,
)

// TfCommandParams represents the input parameters for Terraform-related commands.
// It is used to group Terraform commands for dependency injection.
type TfCommandParams struct {
	fx.In
	Commands []*cli.Command `group:"tf"`
}

// TfCommandResult represents the output result for a Terraform command.
// It is used to group Terraform commands for dependency injection.
type TfCommandResult struct {
	fx.Out
	Command *cli.Command `group:"tf"`
}

// tfCommandsModule defines the Terraform commands module for dependency injection.
var tfCommandsModule = fx.Module("tfCmds",
	fx.Provide(
		NewTfInitCommand,
		NewTfInitAllCommand,
		NewTfApplyCommand,
		NewTfPlanCommand,
		NewTfDestroyCommand,
		NewTfOutputCommand,
		NewTfRefreshCommand,
		NewTfRefreshAllCommand,
		NewTfValidateCommand,
		NewTfFormatCommand,
		NewTfFormatAllCommand,
		NewTfVersionCommand,
	),
)

// AwsCommandParams represents the input parameters for AWS-related commands.
// It is used to group AWS commands for dependency injection.
type AwsCommandParams struct {
	fx.In
	Commands []*cli.Command `group:"aws"`
}

// AwsCommandResult represents the output result for an AWS command.
// It is used to group AWS commands for dependency injection.
type AwsCommandResult struct {
	fx.Out
	Command *cli.Command `group:"aws"`
}

// awsCommandsModule defines the AWS commands module for dependency injection.
var awsCommandsModule = fx.Module("awsCmds",
	fx.Provide(
		NewGetEksTokenCommand,
	),
)
