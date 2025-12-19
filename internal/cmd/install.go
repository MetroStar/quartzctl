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
	"time"

	"github.com/MetroStar/quartzctl/internal/log"
	"github.com/MetroStar/quartzctl/internal/util"
	"github.com/urfave/cli/v3"
)

// NewRootInstallCommand creates the "install" root command for the CLI.
// This command performs a full installation or update of the Quartz system.
//
// Parameters:
//   - p: *CommandParams containing configuration and runtime parameters.
//
// Returns:
//   - RootCommandResult containing the "install" CLI command.
func NewRootInstallCommand(p *CommandParams) RootCommandResult {
	return RootCommandResult{
		Command: &cli.Command{
			Name:  "install",
			Usage: "Perform a full install/update of the system",
			Action: func(ctx context.Context, ccmd *cli.Command) error {
				err := Install(ctx, p)
				if err != nil {
					return err
				}
				util.Hdrf("Installation successful, duration %v", time.Since(p.startTime))
				return nil
			},
		},
	}
}

// NewRootCleanCommand creates the "clean" root command for the CLI.
// This command performs a full cleanup or teardown of the Quartz system.
//
// Parameters:
//   - p: *CommandParams containing configuration and runtime parameters.
//
// Returns:
//   - RootCommandResult containing the "clean" CLI command.
func NewRootCleanCommand(p *CommandParams) RootCommandResult {
	return RootCommandResult{
		Command: &cli.Command{
			Name:  "clean",
			Usage: "Perform a full cleanup/teardown of the system",
			Flags: []cli.Flag{
				&cli.BoolFlag{Name: "refresh", Aliases: []string{"r"}, Usage: "refresh", Value: false},
				&cli.BoolFlag{Name: "force-cleanup", Aliases: []string{"f"}, Usage: "Force AWS resource cleanup before Terraform destroy (detach ENIs, delete SGs)", Value: false},
			},
			Action: func(ctx context.Context, ccmd *cli.Command) error {
				refresh := ccmd.Bool("refresh")
				forceCleanup := ccmd.Bool("force-cleanup")

				err := Clean(ctx, refresh, forceCleanup, p)
				if err != nil {
					return err
				}
				util.Hdrf("Destruction complete, duration %v", time.Since(p.startTime))
				return nil
			},
		},
	}
}

// Install sets up the Quartz environment by initializing and applying all stages.
// This includes preparing the account, creating the Terraform backend, and applying configurations.
//
// Parameters:
//   - ctx: The context for the operation.
//   - p: *CommandParams containing configuration and runtime parameters.
//
// Returns:
//   - error: An error if the installation fails, otherwise nil.
func Install(ctx context.Context, p *CommandParams) error {
	log.Debug("Entering", "command", "install")
	defer log.Debug("Completed", "command", "install")

	Banner()

	err := Confirm(ctx, "Would you like to install Quartz cluster?", p)
	if err != nil {
		// just means the user said no
		return err
	}

	err = PrepareAccount(ctx, p)
	if err != nil {
		return err
	}

	err = TfCreateBackend(ctx, p)
	if err != nil {
		return err
	}

	for _, s := range p.Settings().Config.StagesOrdered() {
		err = TfInit(ctx, s.Id, p)
		if err != nil {
			return err
		}

		err = TfApply(ctx, s.Id, p)
		if err != nil {
			return err
		}
	}

	err = RefreshSecrets(ctx, p)
	if err != nil {
		return err
	}

	err = ClusterInfo(ctx, p)
	if err != nil {
		return err
	}

	return nil
}

// Clean tears down the Quartz environment, including all managed resources and data.
// This includes refreshing Terraform states, destroying resources, and cleaning up.
//
// Parameters:
//   - ctx: The context for the operation.
//   - refresh: A boolean indicating whether to refresh the Terraform state before destruction.
//   - forceCleanup: A boolean indicating whether to run AWS resource cleanup before Terraform destroy.
//   - p: *CommandParams containing configuration and runtime parameters.
//
// Returns:
//   - error: An error if the cleanup fails, otherwise nil.
func Clean(ctx context.Context, refresh bool, forceCleanup bool, p *CommandParams) error {
	log.Debug("Entering", "command", "clean")
	defer log.Debug("Completed", "command", "clean")

	Banner()

	err := Confirm(ctx, "Are you sure? This action will destroy the Quartz cluster, including all managed resources and data.", p)
	if err != nil {
		// just means the user said no
		return nil
	}

	cleanupStart := time.Now()
	stageTiming := make(map[string]time.Duration)

	// Run force cleanup if requested
	if forceCleanup {
		util.Hdr("Force AWS Cleanup")
		forceStart := time.Now()
		err = ForceAWSCleanup(ctx, p)
		stageTiming["force-cleanup"] = time.Since(forceStart)
		if err != nil {
			log.Warn("Force cleanup encountered errors (continuing)", "error", err)
			// Continue despite errors - the goal is to clean up as much as possible
		}
	}

	stages := p.Settings().Config.StagesOrdered()

	// refresh each stage in case local state is out of sync
	initStart := time.Now()
	for _, s := range stages {
		err = TfInit(ctx, s.Id, p)
		if err != nil {
			return err
		}
		if refresh {
			err = TfRefresh(ctx, s.Id, p)
			if err != nil {
				return err
			}
		}
	}
	stageTiming["init-refresh"] = time.Since(initStart)

	// destroy stages in reverse order with retry logic for transient failures
	slices.Reverse(stages)
	for _, s := range stages {
		stageStart := time.Now()
		err = TfDestroyWithRetry(ctx, s.Id, p, 3, 60*time.Second)
		stageTiming["destroy-"+s.Id] = time.Since(stageStart)
		if err != nil {
			printCleanupTimingSummary(stageTiming, time.Since(cleanupStart))
			return err
		}
	}

	backendStart := time.Now()
	err = TfDestroyBackend(ctx, p)
	stageTiming["destroy-backend"] = time.Since(backendStart)
	if err != nil {
		printCleanupTimingSummary(stageTiming, time.Since(cleanupStart))
		return err
	}

	cleanupFinalStart := time.Now()
	err = Cleanup(ctx, p)
	stageTiming["cleanup-final"] = time.Since(cleanupFinalStart)

	printCleanupTimingSummary(stageTiming, time.Since(cleanupStart))
	return err
}

// printCleanupTimingSummary outputs timing information for each phase of the cleanup.
func printCleanupTimingSummary(stageTiming map[string]time.Duration, totalDuration time.Duration) {
	util.Hdr("Cleanup Timing Summary")
	for stage, duration := range stageTiming {
		util.Msgf("  %-25s %v", stage+":", duration.Round(time.Second))
	}
	util.Msgf("  %-25s %v", "TOTAL:", totalDuration.Round(time.Second))
}

// TfDestroyWithRetry attempts to destroy a stage with retry logic for transient failures
// such as AWS resource dependency violations that may resolve after ENI cleanup completes.
//
// Parameters:
//   - ctx: The context for the operation.
//   - stage: The stage ID to destroy.
//   - p: *CommandParams containing configuration and runtime parameters.
//   - maxRetries: Maximum number of retry attempts.
//   - retryDelay: Duration to wait between retries.
//
// Returns:
//   - error: An error if all attempts fail, otherwise nil.
func TfDestroyWithRetry(ctx context.Context, stage string, p *CommandParams, maxRetries int, retryDelay time.Duration) error {
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			log.Info("Retrying destroy after transient failure", "stage", stage, "attempt", attempt, "maxRetries", maxRetries)
			util.Msgf("Waiting %v before retry %d/%d for stage %s...", retryDelay, attempt, maxRetries, stage)
			time.Sleep(retryDelay)
		}

		lastErr = TfDestroy(ctx, stage, p)
		if lastErr == nil {
			return nil
		}

		// Check if this is a retryable error (DependencyViolation typically resolves after waiting)
		errStr := lastErr.Error()
		if !isRetryableDestroyError(errStr) {
			log.Warn("Non-retryable error during destroy", "stage", stage, "error", lastErr)
			return lastErr
		}

		log.Warn("Retryable error during destroy", "stage", stage, "attempt", attempt, "error", lastErr)
	}

	return lastErr
}

// isRetryableDestroyError checks if an error is likely transient and worth retrying.
func isRetryableDestroyError(errStr string) bool {
	retryablePatterns := []string{
		"DependencyViolation",
		"has a dependent object",
		"is currently in use",
		"NetworkInterfaceInUse",
		"InvalidGroup.InUse",
	}
	for _, pattern := range retryablePatterns {
		if len(errStr) > 0 && contains(errStr, pattern) {
			return true
		}
	}
	return false
}

// contains checks if a string contains a substring (case-insensitive would be better but keeping simple)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
