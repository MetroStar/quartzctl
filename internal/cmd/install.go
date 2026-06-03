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
	"fmt"
	"os"
	"path/filepath"
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

	// Load checkpoint to detect previously completed stages
	cp := loadCheckpoint(p)

	for _, s := range stages {
		if cp.isCompleted(s.Id) {
			// A checkpointed stage is not blindly skipped. It may have drifted
			// since it last completed (manual console edits, template/var
			// changes, or a partial prior apply). Run a plan and only skip when
			// the stage is genuinely in sync; otherwise re-apply so a resume
			// converges the cluster instead of silently leaving it stale.
			drifted, derr := stageHasDrift(ctx, s.Id, p)
			if derr != nil {
				// Drift detection is best-effort. If it fails (e.g. transient
				// backend error), preserve the prior fast-resume behavior and
				// skip rather than blocking the whole install.
				util.Msgf("Stage %s already completed; drift check failed (%v), skipping", s.Id, derr)
				continue
			}
			if !drifted {
				util.Msgf("Stage %s already completed and in sync, skipping", s.Id)
				continue
			}
			util.Msgf("Stage %s already completed but drift detected, re-applying", s.Id)
			// Fall through to re-apply below.
		}

		err = TfInit(ctx, s.Id, p)
		if err != nil {
			return err
		}

		err = TfApplyWithRetry(ctx, s.Id, p, 1, 30*time.Second)
		if err != nil {
			// The core stage bootstraps the umbrella Helm release once and then
			// hands ownership to Flux, which re-renders from git and stamps the
			// release version with the git revision (e.g. "1.0.0+<sha>"). Any
			// later apply against a live, Flux-managed cluster sees the bare
			// bootstrap chart "1.0.0" versus Flux's "1.0.0+<sha>" and the Helm
			// provider aborts at plan time with "Planned version is different
			// from configured version". This is expected and NOT a failure:
			// Flux already owns the release, so the bootstrap apply is a no-op
			// (re-applying it would prune Flux's entire rendered stack). Treat
			// the stage as satisfied so the install converges instead of
			// aborting. A genuinely fresh install never hits this branch because
			// Flux has not yet adopted the release.
			if isFluxOwnedReleaseDrift(err) {
				log.Debug("Apply reported Flux-owned release version drift; treating stage as complete",
					"stage", s.Id, "error", err)
				util.Msgf("Stage %s is owned by Flux (release already adopted), skipping bootstrap apply", s.Id)
				cp.markCompleted(s.Id)
				cp.save(p)
				continue
			}
			return err
		}

		cp.markCompleted(s.Id)
		cp.save(p)
	}

	// Clear checkpoint on successful completion
	cp.clear(p)

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
	// Uses TfRefreshWithUnlock for automatic state lock recovery.
	initStart := time.Now()
	for _, s := range stages {
		err = TfInit(ctx, s.Id, p)
		if err != nil {
			log.Warn("Init failed for stage, will attempt destroy anyway", "stage", s.Id, "error", err)
		}
		err = TfRefreshWithUnlock(ctx, s.Id, p)
		if err != nil {
			// The umbrella Helm release is version-stamped by Flux ("1.0.0+<sha>")
			// once it adopts the bootstrap release, so a pre-destroy refresh of
			// the core stage surfaces the benign Helm provider "Planned version
			// is different from configured version" mismatch. The subsequent
			// destroy runs with Refresh(false) and is unaffected, so this is
			// cosmetic — demote it to debug to avoid alarming clean output.
			if isFluxOwnedReleaseDrift(err) {
				log.Debug("Refresh reported benign Flux-owned release version drift, continuing", "stage", s.Id, "error", err)
			} else {
				log.Warn("Refresh failed for stage", "stage", s.Id, "error", err)
			}
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

	// Destroy stages in parallel where possible. Stages are grouped into
	// "destroy waves" based on reverse dependencies: stages with no dependents
	// can be destroyed first (highest order numbers), then their dependencies.
	destroyWaves := buildDestroyWaves(stages)
	for waveIdx, wave := range destroyWaves {
		if len(wave) == 1 {
			// Single stage — no parallelism needed
			s := wave[0]
			stageStart := time.Now()
			err = TfDestroyWithRetry(ctx, s.Id, p, 2, 30*time.Second)
			stageTiming["destroy-"+s.Id] = time.Since(stageStart)
			if err != nil {
				log.Warn("Stage destroy failed, continuing with remaining stages", "stage", s.Id, "error", err)
				destroyErrors = append(destroyErrors, fmt.Errorf("stage %s: %w", s.Id, err))
			}
		} else {
			// Multiple independent stages — destroy in parallel
			util.Msgf("Destroying %d stages in parallel (wave %d/%d): %s", len(wave), waveIdx+1, len(destroyWaves), stageIds(wave))
			waveStart := time.Now()
			waveErrors := parallelDestroy(ctx, wave, p)
			for _, s := range wave {
				stageTiming["destroy-"+s.Id] = time.Since(waveStart)
			}
			destroyErrors = append(destroyErrors, waveErrors...)
		}

		// Inter-wave K8s cleanup
		if kube != nil && waveIdx < len(destroyWaves)-1 {
			interStart := time.Now()
			if cleanErr := kube.InterStageCleanup(ctx); cleanErr != nil {
				log.Warn("Inter-stage K8s cleanup failed (non-fatal)", "error", cleanErr)
			}
			stageTiming[fmt.Sprintf("k8s-inter-wave-%d", waveIdx)] = time.Since(interStart)
		}
	}

	// Only destroy the state backend if ALL stage destroys succeeded.
	// If any stage failed, the backend must remain intact so operators can
	// re-run clean or use tofu commands to recover. Destroying the backend
	// with failed stages makes the orphaned resources irrecoverable via tofu.
	if len(destroyErrors) == 0 {
		backendStart := time.Now()
		err = TfDestroyBackend(ctx, p)
		stageTiming["destroy-backend"] = time.Since(backendStart)
		if err != nil {
			destroyErrors = append(destroyErrors, fmt.Errorf("backend: %w", err))
		}
	} else {
		util.Msgf("⚠️  Skipping backend destruction — %d stage(s) failed. Re-run 'quartz clean' to retry.", len(destroyErrors))
		util.Msg("   The state backend is preserved so tofu can still manage remaining resources.")
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

		errStr := lastErr.Error()

		// Attempt automatic stale lock recovery
		if strings.Contains(errStr, "Error acquiring the state lock") || strings.Contains(errStr, "state blob is already locked") {
			recoverStaleLock(ctx, stage, p, errStr)
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
// Release lifecycle is managed by Flux remediation; quartzctl retries only infra transients.
func isRetryableDestroyError(errStr string) bool {
	retryablePatterns := []string{
		"DependencyViolation",
		"has a dependent object",
		"is currently in use",
		"NetworkInterfaceInUse",
		"InvalidGroup.InUse",
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

		// Attempt automatic stale lock recovery
		if strings.Contains(errStr, "Error acquiring the state lock") || strings.Contains(errStr, "state blob is already locked") {
			recoverStaleLock(ctx, stage, p, errStr)
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
// Application-level lifecycle errors (MissingRollbackTarget, upgrade retries) are handled
// declaratively by Flux remediation in the chart; quartzctl only retries infra transients.
func isRetryableApplyError(errStr string) bool {
	retryablePatterns := []string{
		"Error acquiring the state lock",
		"state blob is already locked",
		"Kubernetes cluster unreachable",
		"connection refused",
		"no endpoints available",
		"i/o timeout",
		"timeout while waiting for state to become",
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

// recoverStaleLock examines a state lock error, extracts the lock ID,
// and force-unlocks it. During retry loops, locks are always from our own
// prior failed attempt — no age check needed.
// Returns true if the lock was successfully recovered.
func recoverStaleLock(ctx context.Context, stage string, p *CommandParams, errStr string) bool {
	lockID, ok := tofu.ExtractLockID(errStr)
	if !ok {
		return false
	}

	log.Warn("State lock detected, force-unlocking", "stage", stage, "lockID", lockID)

	client := tofu.Instance(ctx, *p.Settings())
	s := p.Settings().Config.Stages[stage]
	if unlockErr := client.ForceUnlock(ctx, s, lockID); unlockErr != nil {
		log.Warn("Force-unlock failed", "stage", stage, "lockID", lockID, "error", unlockErr)
		return false
	}

	util.Msgf("Successfully force-unlocked state for stage %s (lock ID: %s)", stage, lockID)
	return true
}

// buildDestroyWaves groups stages into waves for parallel destruction.
// Stages in the same wave have no dependency relationships between them.
// Waves are ordered so that dependents are destroyed before their dependencies
// (highest-order stages first).
func buildDestroyWaves(stages []schema.StageConfig) [][]schema.StageConfig {
	// Build a set of stage IDs for quick lookup
	stageMap := make(map[string]schema.StageConfig)
	for _, s := range stages {
		stageMap[s.Id] = s
	}

	// Group stages by order (stages with the same order are independent)
	orderGroups := make(map[int][]schema.StageConfig)
	var orders []int
	for _, s := range stages {
		if _, exists := orderGroups[s.Order]; !exists {
			orders = append(orders, s.Order)
		}
		orderGroups[s.Order] = append(orderGroups[s.Order], s)
	}

	// Sort orders descending (highest first = destroy dependents before dependencies)
	slices.Sort(orders)
	slices.Reverse(orders)

	var waves [][]schema.StageConfig
	for _, order := range orders {
		waves = append(waves, orderGroups[order])
	}

	return waves
}

// parallelDestroy destroys multiple independent stages concurrently.
// Returns a slice of errors from any stages that failed.
func parallelDestroy(ctx context.Context, stages []schema.StageConfig, p *CommandParams) []error {
	type result struct {
		stageId string
		err     error
	}

	results := make(chan result, len(stages))

	for _, s := range stages {
		go func(stage schema.StageConfig) {
			err := TfDestroyWithRetry(ctx, stage.Id, p, 2, 30*time.Second)
			results <- result{stageId: stage.Id, err: err}
		}(s)
	}

	var errs []error
	for range stages {
		r := <-results
		if r.err != nil {
			log.Warn("Stage destroy failed in parallel wave", "stage", r.stageId, "error", r.err)
			errs = append(errs, fmt.Errorf("stage %s: %w", r.stageId, r.err))
		}
	}

	return errs
}

// stageHasDrift initializes the given stage and runs a plan to detect whether
// the live infrastructure has drifted from the desired configuration. It is
// used on resume to decide whether an already-checkpointed stage can be safely
// skipped or must be re-applied. Returns true when the plan contains changes.
func stageHasDrift(ctx context.Context, stage string, p *CommandParams) (bool, error) {
	if err := TfInit(ctx, stage, p); err != nil {
		return false, err
	}

	if err := tfStagePrep(ctx, stage, p); err != nil {
		return false, err
	}

	client := tofu.Instance(ctx, *p.Settings())
	s := p.Settings().Config.Stages[stage]
	hasChanges, err := client.Plan(ctx, s)
	if err != nil {
		// The core stage bootstraps the umbrella Helm release once and then
		// hands ownership to Flux, which re-renders the chart from git and
		// stamps the release version with the git revision as semver build
		// metadata (e.g. "1.0.0+<sha>"). The local bootstrap chart is bare
		// "1.0.0", so the Helm provider can never reconcile the two: it raises
		// "Planned version is different from configured version" at plan time.
		// This is NOT actionable drift — Flux solely owns the release after
		// bootstrap — so treat it as in-sync and skip rather than failing the
		// drift check or (worse) re-applying the bootstrap render, which would
		// prune Flux's entire rendered stack.
		if isFluxOwnedReleaseDrift(err) {
			log.Debug("Plan reported Flux-owned release version drift; treating stage as in sync",
				"stage", stage, "error", err)
			return false, nil
		}
		return false, err
	}
	return hasChanges, nil
}

// isFluxOwnedReleaseDrift reports whether a plan error is the benign Helm
// provider version-mismatch that occurs when OpenTofu's bootstrap Helm release
// has since been adopted and re-versioned by Flux. Such drift is expected and
// must not trigger a destructive re-apply on resume.
func isFluxOwnedReleaseDrift(err error) bool {
	return err != nil &&
		strings.Contains(err.Error(), "Planned version is different from configured version")
}

// checkpoint tracks which stages have completed successfully during an install.
// On re-entry after a failure, completed stages are skipped for idempotent behavior.
const checkpointFileName = "install-checkpoint.json"

type checkpoint struct {
	Stages map[string]time.Time `json:"stages"`
}

func checkpointPath(p *CommandParams) string {
	return filepath.Join(p.Settings().Config.Tmp, checkpointFileName)
}

func loadCheckpoint(p *CommandParams) *checkpoint {
	cp := &checkpoint{Stages: make(map[string]time.Time)}
	data, err := os.ReadFile(checkpointPath(p))
	if err != nil {
		return cp
	}
	if err := json.Unmarshal(data, cp); err != nil {
		log.Warn("Corrupt checkpoint file, starting fresh", "error", err)
		return &checkpoint{Stages: make(map[string]time.Time)}
	}
	if len(cp.Stages) > 0 {
		util.Msgf("Found install checkpoint with %d completed stage(s)", len(cp.Stages))
	}
	return cp
}

func (cp *checkpoint) isCompleted(stageId string) bool {
	_, ok := cp.Stages[stageId]
	return ok
}

func (cp *checkpoint) markCompleted(stageId string) {
	cp.Stages[stageId] = time.Now()
}

func (cp *checkpoint) save(p *CommandParams) {
	data, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		log.Warn("Failed to marshal checkpoint", "error", err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(checkpointPath(p)), 0750); err != nil {
		log.Warn("Failed to create checkpoint directory", "error", err)
		return
	}
	if err := os.WriteFile(checkpointPath(p), data, 0600); err != nil {
		log.Warn("Failed to write checkpoint", "error", err)
	}
}

func (cp *checkpoint) clear(p *CommandParams) {
	os.Remove(checkpointPath(p))
}
