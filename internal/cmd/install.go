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
	"fmt"
	"slices"
	"strings"
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
				&cli.BoolFlag{Name: "refresh", Aliases: []string{"r"}, Usage: "refresh (always enabled)", Value: true},
			},
			Action: func(ctx context.Context, ccmd *cli.Command) error {
				err := Clean(ctx, p)
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
// This includes preparing the account, creating the OpenTofu backend, and applying configurations.
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
// This includes refreshing OpenTofu states, destroying resources, and cleaning up.
// Errors during individual stage destroys are collected and reported but do not
// abort remaining stages, ensuring maximum cleanup even with partial failures.
//
// Parameters:
//   - ctx: The context for the operation.
//   - p: *CommandParams containing configuration and runtime parameters.
//
// Returns:
//   - error: An error if the cleanup fails, otherwise nil.
func Clean(ctx context.Context, p *CommandParams) error {
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
	var destroyErrors []error

	stages := p.Settings().Config.StagesOrdered()

	// Initialize and refresh each stage before destruction
	initStart := time.Now()
	for _, s := range stages {
		err = TfInit(ctx, s.Id, p)
		if err != nil {
			log.Warn("Init failed for stage, will attempt destroy anyway", "stage", s.Id, "error", err)
		}
		// Always refresh state before destroy to detect drift
		err = TfRefresh(ctx, s.Id, p)
		if err != nil {
			log.Warn("Refresh failed for stage", "stage", s.Id, "error", err)
		}
	}
	stageTiming["init-refresh"] = time.Since(initStart)

	// Destroy stages in reverse order, collecting errors instead of aborting
	slices.Reverse(stages)
	for _, s := range stages {
		stageStart := time.Now()
		err = TfDestroyWithRetry(ctx, s.Id, p, 2, 30*time.Second)
		stageTiming["destroy-"+s.Id] = time.Since(stageStart)
		if err != nil {
			log.Warn("Stage destroy failed, continuing with remaining stages", "stage", s.Id, "error", err)
			destroyErrors = append(destroyErrors, fmt.Errorf("stage %s: %w", s.Id, err))
		}
	}

	backendStart := time.Now()
	err = TfDestroyBackend(ctx, p)
	stageTiming["destroy-backend"] = time.Since(backendStart)
	if err != nil {
		destroyErrors = append(destroyErrors, fmt.Errorf("backend: %w", err))
	}

	cleanupFinalStart := time.Now()
	err = Cleanup(ctx, p)
	stageTiming["cleanup-final"] = time.Since(cleanupFinalStart)
	if err != nil {
		destroyErrors = append(destroyErrors, fmt.Errorf("cleanup: %w", err))
	}

	printCleanupTimingSummary(stageTiming, time.Since(cleanupStart))

	if len(destroyErrors) > 0 {
		util.Hdr("Destroy Errors")
		for _, e := range destroyErrors {
			util.Msgf("  ✗ %v", e)
		}
		return fmt.Errorf("%d stage(s) failed to destroy cleanly", len(destroyErrors))
	}

	return nil
}

// printCleanupTimingSummary outputs timing information for each phase of the cleanup.
func printCleanupTimingSummary(stageTiming map[string]time.Duration, totalDuration time.Duration) {
	util.Hdr("Cleanup Timing Summary")
	for stage, duration := range stageTiming {
		util.Msgf("  %-25s %v", stage+":", duration.Round(time.Second))
	}
	util.Msgf("  %-25s %v", "TOTAL:", totalDuration.Round(time.Second))
}

// TfDestroyWithRetry attempts to destroy a stage with retry logic for transient failures.
// It retries on known transient errors (dependency violations, timeouts) with a simple
// delay between attempts. Cleanup of blocking resources is handled by Helm pre-delete
// hooks running in-cluster.
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

		// Check if this is a retryable error
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
		"failed to delete release",
		"Kubernetes cluster unreachable",
		"connection refused",
		"no endpoints available",
		"i/o timeout",
	}
	for _, pattern := range retryablePatterns {
		if len(errStr) > 0 && strings.Contains(errStr, pattern) {
			return true
		}
	}
	return false
}
