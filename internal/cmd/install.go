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
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/MetroStar/quartzctl/internal/config/schema"
	"github.com/MetroStar/quartzctl/internal/log"
	"github.com/MetroStar/quartzctl/internal/tofu"
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
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "resume-from", Aliases: []string{"r"}, Usage: "Resume installation from a specific stage ID (skips earlier stages)"},
				&cli.BoolFlag{Name: "allow-deferral", Usage: "Enable OpenTofu deferred actions for resources that cannot be fully resolved in one pass"},
			},
			Action: func(ctx context.Context, ccmd *cli.Command) error {
				resumeFrom := ccmd.String("resume-from")
				allowDeferral := ccmd.Bool("allow-deferral")
				p.allowDeferral = allowDeferral
				err := Install(ctx, p, resumeFrom)
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
func Install(ctx context.Context, p *CommandParams, resumeFrom string) error {
	log.Debug("Entering", "command", "install")
	defer log.Debug("Completed", "command", "install")

	Banner()

	err := Confirm(ctx, "Would you like to install Quartz cluster?", p)
	if err != nil {
		// just means the user said no
		return err
	}

	// Preflight validation: check IAM credentials and cloud connectivity
	err = Preflight(ctx, p)
	if err != nil {
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

	stages := p.Settings().Config.StagesOrdered()

	// If --resume-from is specified, skip stages until we reach the target
	if resumeFrom != "" {
		found := false
		for i, s := range stages {
			if strings.EqualFold(s.Id, resumeFrom) {
				stages = stages[i:]
				found = true
				util.Msgf("Resuming installation from stage: %s", resumeFrom)
				break
			}
		}
		if !found {
			return fmt.Errorf("stage %q not found; available stages: %s", resumeFrom, stageIds(p.Settings().Config.StagesOrdered()))
		}
	}

	// Group stages by order for parallel execution within the same order level
	stageGroups := groupStagesByOrder(stages)
	for _, group := range stageGroups {
		if len(group) == 1 {
			// Single stage in this order - run sequentially
			s := group[0]
			err = TfInit(ctx, s.Id, p)
			if err != nil {
				return err
			}
			err = TfApplyWithRetry(ctx, s.Id, p, 2, 30*time.Second)
			if err != nil {
				return err
			}
		} else {
			// Multiple stages at the same order - run in parallel
			err = applyStagesParallel(ctx, group, p)
			if err != nil {
				return err
			}
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

// stageIds returns a comma-separated list of stage IDs for error messages.
func stageIds(stages []schema.StageConfig) string {
	ids := make([]string, len(stages))
	for i, s := range stages {
		ids[i] = s.Id
	}
	return strings.Join(ids, ", ")
}

// groupStagesByOrder groups stages by their Order field, preserving sort order.
// Stages with the same order can run in parallel.
func groupStagesByOrder(stages []schema.StageConfig) [][]schema.StageConfig {
	if len(stages) == 0 {
		return nil
	}

	var groups [][]schema.StageConfig
	var current []schema.StageConfig
	currentOrder := stages[0].Order

	for _, s := range stages {
		if s.Order != currentOrder {
			groups = append(groups, current)
			current = nil
			currentOrder = s.Order
		}
		current = append(current, s)
	}
	if len(current) > 0 {
		groups = append(groups, current)
	}

	return groups
}

// applyStagesParallel runs init+apply for multiple stages concurrently.
// Returns the first error encountered (all stages are attempted).
func applyStagesParallel(ctx context.Context, stages []schema.StageConfig, p *CommandParams) error {
	ids := make([]string, len(stages))
	for i, s := range stages {
		ids[i] = s.Id
	}
	util.Msgf("Running stages in parallel: %s", strings.Join(ids, ", "))

	type result struct {
		stageId string
		err     error
	}

	ch := make(chan result, len(stages))
	for _, s := range stages {
		go func(s schema.StageConfig) {
			var stageErr error
			stageErr = TfInit(ctx, s.Id, p)
			if stageErr == nil {
				stageErr = TfApplyWithRetry(ctx, s.Id, p, 2, 30*time.Second)
			}
			ch <- result{stageId: s.Id, err: stageErr}
		}(s)
	}

	var errs []error
	for range stages {
		r := <-ch
		if r.err != nil {
			errs = append(errs, fmt.Errorf("stage %s: %w", r.stageId, r.err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("parallel stage execution failed: %w", errors.Join(errs...))
	}
	return nil
}

// Preflight validates cloud credentials and connectivity before starting the install.
// Fails fast if IAM credentials are invalid or the cloud provider is unreachable.
func Preflight(ctx context.Context, p *CommandParams) error {
	util.Hdr("Preflight Checks")

	cp, err := p.Provider().Cloud(ctx)
	if err != nil {
		return fmt.Errorf("preflight: failed to initialize cloud provider: %w", err)
	}

	result := cp.CheckAccess(ctx)
	headers, rows := result.ToTable()

	// Check for any failing rows
	for _, row := range rows {
		if row.Error != nil {
			return fmt.Errorf("preflight: cloud access check failed: %w", row.Error)
		}
	}

	// Print identity summary
	if len(rows) > 0 && len(headers) == len(rows[0].Data) {
		for i, h := range headers {
			util.Msgf("  %-15s %s", h+":", rows[0].Data[i])
		}
	}

	util.Msg("  Preflight checks passed")
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

	// Initialize and refresh each stage before destruction.
	// Uses TfRefreshWithUnlock for automatic state lock recovery, and
	// skips refresh for service-dependent stages when their provider endpoint is unreachable.
	initStart := time.Now()
	for _, s := range stages {
		err = TfInit(ctx, s.Id, p)
		if err != nil {
			log.Warn("Init failed for stage, will attempt destroy anyway", "stage", s.Id, "error", err)
		}
		// Skip refresh for service-dependent stages if their service is unreachable.
		// These stages (sonarqube, keycloak) require their respective services running
		// for the provider to connect. During clean, services may already be gone.
		if isServiceDependentStage(s.Id) {
			log.Info("Skipping refresh for service-dependent stage (will use -refresh=false on destroy)", "stage", s.Id)
			continue
		}
		err = TfRefreshWithUnlock(ctx, s.Id, p)
		if err != nil {
			log.Warn("Refresh failed for stage", "stage", s.Id, "error", err)
		}
	}
	stageTiming["init-refresh"] = time.Since(initStart)

	// K8s preparation: remove Flux finalizers, patch stuck namespaces,
	// and clean up resources that would block destroy
	k8sCleanStart := time.Now()
	kube, err := p.Provider().Kubernetes(ctx)
	if err != nil {
		log.Warn("Could not connect to Kubernetes for pre-destroy cleanup", "error", err)
	} else {
		err = kube.PrepareForDestroy(ctx)
		if err != nil {
			log.Warn("K8s pre-destroy cleanup failed (non-fatal)", "error", err)
		}
	}
	stageTiming["k8s-prep"] = time.Since(k8sCleanStart)

	// Destroy stages in reverse order, collecting errors instead of aborting.
	// Service-dependent stages are destroyed with -refresh=false since their
	// provider endpoints are unreachable. Empty states are auto-skipped by tofu.
	slices.Reverse(stages)
	for i, s := range stages {
		stageStart := time.Now()

		// For service-dependent stages, the provider can't connect so destroy
		// would fail on provider initialization. Remove resources from state instead.
		if isServiceDependentStage(s.Id) {
			util.Hdrf("Destroy %s (service-dependent — clearing state)", s.Id)
			client := tofu.Instance(ctx, *p.Settings())
			stateErr := client.StateClear(ctx, s)
			stageTiming["destroy-"+s.Id] = time.Since(stageStart)
			if stateErr != nil {
				log.Warn("Failed to clear state for service-dependent stage", "stage", s.Id, "error", stateErr)
				destroyErrors = append(destroyErrors, fmt.Errorf("stage %s: %w", s.Id, stateErr))
			}
			continue
		}

		err = TfDestroyWithRetry(ctx, s.Id, p, 2, 30*time.Second)
		stageTiming["destroy-"+s.Id] = time.Since(stageStart)
		if err != nil {
			log.Warn("Stage destroy failed, continuing with remaining stages", "stage", s.Id, "error", err)
			destroyErrors = append(destroyErrors, fmt.Errorf("stage %s: %w", s.Id, err))
		}

		// Inter-stage K8s cleanup: patch newly-stuck finalizers, delete orphaned
		// webhooks, and force-delete Terminating pods between stage destroys.
		// This catches issues that emerge AFTER a stage is destroyed (e.g., LB
		// controller destroyed in stage 11 leaves services with stuck finalizers).
		if kube != nil && i < len(stages)-1 {
			interStart := time.Now()
			if cleanErr := kube.InterStageCleanup(ctx); cleanErr != nil {
				log.Warn("Inter-stage K8s cleanup failed (non-fatal)", "error", cleanErr)
			}
			stageTiming["k8s-inter-"+s.Id] = time.Since(interStart)
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

// isServiceDependentStage returns true for stages that require an external service
// endpoint to be reachable for their provider to initialize (e.g., sonarqube needs
// the SonarQube server, keycloak needs the Keycloak server).
func isServiceDependentStage(stageID string) bool {
	serviceDependentStages := []string{"sonarqube", "keycloak"}
	for _, s := range serviceDependentStages {
		if strings.Contains(stageID, s) {
			return true
		}
	}
	return false
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

		errStr := lastErr.Error()

		// Attempt automatic state lock recovery
		if strings.Contains(errStr, "Error acquiring the state lock") || strings.Contains(errStr, "state blob is already locked") {
			if lockID, ok := tofu.ExtractLockID(errStr); ok {
				client := tofu.Instance(ctx, *p.Settings())
				s := p.Settings().Config.Stages[stage]
				if unlockErr := client.ForceUnlock(ctx, s, lockID); unlockErr != nil {
					log.Warn("Force-unlock failed", "stage", stage, "lockID", lockID, "error", unlockErr)
				} else {
					util.Msgf("Successfully force-unlocked state for stage %s (lock ID: %s)", stage, lockID)
				}
			}
		}

		// Check if this is a retryable error
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
		"Error acquiring the state lock",
		"state blob is already locked",
	}
	for _, pattern := range retryablePatterns {
		if len(errStr) > 0 && strings.Contains(errStr, pattern) {
			return true
		}
	}
	return false
}

// TfApplyWithRetry attempts to apply a stage with retry logic for transient failures.
// It retries on known transient errors (state locks, timeouts, API throttling) with
// exponential backoff between attempts.
func TfApplyWithRetry(ctx context.Context, stage string, p *CommandParams, maxRetries int, retryDelay time.Duration) error {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff: retryDelay * 2^(attempt-1)
			backoff := retryDelay * time.Duration(1<<(attempt-1))
			log.Info("Retrying apply after transient failure", "stage", stage, "attempt", attempt, "maxRetries", maxRetries)
			util.Msgf("Waiting %v before retry %d/%d for stage %s...", backoff, attempt, maxRetries, stage)
			time.Sleep(backoff)
		}

		lastErr = TfApply(ctx, stage, p)
		if lastErr == nil {
			return nil
		}

		errStr := lastErr.Error()

		// Attempt automatic state lock recovery
		if strings.Contains(errStr, "Error acquiring the state lock") || strings.Contains(errStr, "state blob is already locked") {
			if lockID, ok := tofu.ExtractLockID(errStr); ok {
				client := tofu.Instance(ctx, *p.Settings())
				s := p.Settings().Config.Stages[stage]
				if unlockErr := client.ForceUnlock(ctx, s, lockID); unlockErr != nil {
					log.Warn("Force-unlock failed", "stage", stage, "lockID", lockID, "error", unlockErr)
				} else {
					util.Msgf("Successfully force-unlocked state for stage %s (lock ID: %s)", stage, lockID)
				}
			}
		}

		if !isRetryableApplyError(errStr) {
			log.Warn("Non-retryable error during apply", "stage", stage, "error", lastErr)
			return lastErr
		}

		log.Warn("Retryable error during apply", "stage", stage, "attempt", attempt, "error", lastErr)
	}

	return lastErr
}

// isRetryableApplyError checks if an apply error is likely transient and worth retrying.
func isRetryableApplyError(errStr string) bool {
	retryablePatterns := []string{
		"Error acquiring the state lock",
		"state blob is already locked",
		"Kubernetes cluster unreachable",
		"connection refused",
		"no endpoints available",
		"i/o timeout",
		"timeout while waiting for state to become",
		"error creating",
		"TooManyRequestsException",
		"Throttling",
		"RequestLimitExceeded",
		"ServiceUnavailable",
		"context deadline exceeded",
	}
	for _, pattern := range retryablePatterns {
		if len(errStr) > 0 && strings.Contains(errStr, pattern) {
			return true
		}
	}
	return false
}
