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
	"sync"
	"time"

	"github.com/MetroStar/quartzctl/internal/config/schema"
	"github.com/MetroStar/quartzctl/internal/log"
	"github.com/MetroStar/quartzctl/internal/provider"
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
				&cli.BoolFlag{Name: "yes", Aliases: []string{"y"}, Usage: "Skip the interactive confirmation prompt (assume yes)"},
			},
			Action: func(ctx context.Context, ccmd *cli.Command) error {
				resumeFrom := ccmd.String("resume-from")
				allowDeferral := ccmd.Bool("allow-deferral")
				p.allowDeferral = allowDeferral
				p.assumeYes = ccmd.Bool("yes")
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
				&cli.BoolFlag{Name: "yes", Aliases: []string{"y"}, Usage: "Skip the interactive confirmation prompt (assume yes)"},
			},
			Action: func(ctx context.Context, ccmd *cli.Command) error {
				p.assumeYes = ccmd.Bool("yes")
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

		err = TfApplyWithRetry(ctx, s.Id, p, 2, 30*time.Second)
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

				// The bootstrap apply is a no-op, but the stage's co-resources
				// (e.g. the values overlay Secret consumed by the release via
				// valuesFrom) still need to converge so day-2 quartz.yaml changes
				// take effect. An untargeted apply can't reach them: the Helm
				// provider aborts the whole plan on the version mismatch before
				// any resource applies. Fall back to a targeted apply of the
				// stage's declared convergence resources, which excludes the
				// Flux-owned release and so avoids the abort.
				if targets := s.Flux.ConvergeTargets; len(targets) > 0 {
					if cerr := TfApplyTargeted(ctx, s.Id, p, targets); cerr != nil {
						log.Warn("Targeted convergence apply failed for Flux-owned stage",
							"stage", s.Id, "targets", targets, "error", cerr)
						return cerr
					}
					util.Msgf("Stage %s co-resources converged via targeted apply", s.Id)
				}

				cp.markCompleted(s.Id)
				cp.save(p)
				continue
			}
			return err
		}

		cp.markCompleted(s.Id)
		cp.save(p)
	}

	err = RefreshSecrets(ctx, p)
	if err != nil {
		return err
	}

	// Final convergence gate. Per-stage post-checks only verify that stage's
	// own HelmRelease, so without this an install can report success while
	// sibling releases are still failing to reconcile. Wait for every Flux
	// HelmRelease to become Ready (or fail with the offending releases named)
	// before declaring the install successful. Runs after RefreshSecrets so
	// external-secret-dependent releases have their inputs in place. The
	// checkpoint is intentionally cleared only AFTER this gate passes so a
	// failed convergence still allows a fast drift-aware resume.
	err = waitForClusterConvergence(ctx, p)
	if err != nil {
		return err
	}

	// Clear checkpoint on successful completion.
	cp.clear(p)

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

// helmConvergenceTimeout returns the overall budget for the post-install
// HelmRelease convergence gate, and whether it was set explicitly by the
// operator. Defaults to 30m; override with QUARTZ_CONVERGENCE_TIMEOUT (e.g.
// "45m"). Set to "0" to disable the gate.
//
// When NOT set explicitly, the returned value is only a floor: the gate raises
// it to fit the slowest HelmRelease's own spec.timeout (see adaptiveConvergenceTimeout),
// because a release that Flux legitimately allows 90m must not be failed by a
// gate that only waits 30m. An explicit value is always honored verbatim.
func helmConvergenceTimeout() (time.Duration, bool) {
	const def = 30 * time.Minute
	if v := os.Getenv("QUARTZ_CONVERGENCE_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d >= 0 {
			return d, true
		}
	}
	return def, false
}

// adaptiveConvergenceTimeout sizes the gate's wait to the slowest release. A
// release with spec.timeout T may consume the whole T on its first attempt and
// then remediate (retry) once before genuinely converging; the resulting
// workloads then need the stabilize grace to roll out. Budgeting 2*T + grace
// lets a healthy-but-slow release (cold image pulls, model warmers) finish
// instead of tripping a misleading "did not converge" while it is still
// progressing. The static floor still applies for fast clusters. The gate
// exits the instant all releases are Ready or any release Stalls, so a generous
// backstop never makes a converged install wait.
func adaptiveConvergenceTimeout(floor, maxReleaseTimeout, grace time.Duration) time.Duration {
	if maxReleaseTimeout <= 0 {
		return floor
	}
	adaptive := 2*maxReleaseTimeout + grace
	if adaptive > floor {
		return adaptive
	}
	return floor
}

// convergencePollInterval returns the cadence for the convergence gate's
// polling loop. Defaults to 20s; override with QUARTZ_CONVERGENCE_INTERVAL.
func convergencePollInterval() time.Duration {
	const def = 20 * time.Second
	if v := os.Getenv("QUARTZ_CONVERGENCE_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return def
}

// workloadStabilizeGrace returns how long, after every HelmRelease has become
// Ready, the convergence gate will wait for the resulting workloads to finish
// rolling out before declaring success. A HelmRelease reports Ready when Helm's
// install/upgrade succeeds, which does NOT guarantee the pods it created are
// healthy: a chart whose RBAC or image is mismatched installs cleanly yet
// crashloops. This grace window lets the gate catch that class of failure
// instead of reporting a misleading success the instant releases are Ready.
// Defaults to 3m; override with QUARTZ_WORKLOAD_GRACE. Set to "0" to disable
// the workload check (HelmRelease readiness alone then ends the gate).
func workloadStabilizeGrace() time.Duration {
	const def = 3 * time.Minute
	if v := os.Getenv("QUARTZ_WORKLOAD_GRACE"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d >= 0 {
			return d
		}
	}
	return def
}

// formatReleaseFailures renders a compact, operator-actionable description of a
// set of not-ready/stalled HelmReleases (id + truncated condition message).
func formatReleaseFailures(rs []provider.HelmReleaseStatus) string {
	if len(rs) == 0 {
		return "(none)"
	}
	parts := make([]string, 0, len(rs))
	for _, r := range rs {
		msg := r.ReadyMsg
		if r.Stalled && r.StalledMsg != "" {
			msg = r.StalledMsg
		}
		msg = strings.TrimSpace(strings.ReplaceAll(msg, "\n", " "))
		if len(msg) > 200 {
			msg = msg[:200] + "..."
		}
		if msg == "" {
			parts = append(parts, r.ID())
		} else {
			parts = append(parts, fmt.Sprintf("%s (%s)", r.ID(), msg))
		}
	}
	return strings.Join(parts, "; ")
}

// waitForClusterConvergence is the final install gate. Per-stage post-checks
// only verify that stage's own HelmRelease, so an install can otherwise report
// success while sibling releases (neuvector, kiali, ...) are still failing to
// reconcile. This polls ALL Flux HelmReleases until every one is Ready, the
// overall timeout elapses, or a release is Stalled (Flux exhausted its
// retries). On a non-convergent outcome it returns an error naming the
// offending releases so the operator gets an actionable failure instead of a
// misleading "Installation successful".
func waitForClusterConvergence(ctx context.Context, p *CommandParams) error {
	timeout, explicit := helmConvergenceTimeout()
	if timeout == 0 {
		util.Msg("HelmRelease convergence gate disabled (QUARTZ_CONVERGENCE_TIMEOUT=0)")
		return nil
	}

	kube, err := p.Provider().Kubernetes(ctx)
	if err != nil {
		// Cluster unreachable. The stage applies already fail loudly if the
		// cluster never came up, so don't manufacture a new failure here.
		log.Debug("Convergence gate: cluster not reachable, skipping", "err", err)
		return nil
	}

	util.Hdr("Waiting for all HelmReleases to converge")

	start := time.Now()
	deadline := start.Add(timeout)
	interval := convergencePollInterval()

	// Unless the operator pinned the budget explicitly, the gate adapts its
	// deadline once to the slowest release's own spec.timeout (computed from the
	// first snapshot below), so it never gives up on a release that is still
	// inside the time Flux is allowed to spend on it.
	deadlineAdapted := explicit

	// A release must report Stalled across consecutive polls before the gate
	// gives up on it, so a brief self-healing blip doesn't abort an
	// otherwise-converging install.
	const stalledThreshold = 2
	stalledStreak := map[string]int{}

	// Once every HelmRelease is Ready, workloads get a bounded grace window to
	// finish rolling out. A pod must stay unhealthy across several consecutive
	// polls before it counts against the install, so transient image-pull or
	// startup churn doesn't produce a false failure.
	workloadGrace := workloadStabilizeGrace()
	const unhealthyThreshold = 3
	unhealthyStreak := map[string]int{}
	var releasesReadyAt time.Time

	var last provider.ClusterProgress
	for {
		snap, sErr := kube.ClusterProgressSnapshot(ctx)
		if sErr != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			// Transient API hiccup — keep trying until the deadline.
			log.Debug("Convergence gate: snapshot failed (will retry)", "err", sErr)
		} else {
			last = snap

			// No HelmReleases present at all (CRD absent or none created yet).
			// There is nothing to converge — don't spin until the deadline. This
			// also keeps the gate a no-op for non-Flux/mock clusters.
			if snap.HelmReleasesTotal == 0 {
				log.Debug("Convergence gate: no HelmReleases present, nothing to wait for")
				return nil
			}

			// Size the wait to the slowest release the first time we can see the
			// releases. Done once: the release set is stable for an install and
			// extending the deadline mid-wait should reflect declared intent, not
			// drift in transient status.
			if !deadlineAdapted {
				deadlineAdapted = true
				if adapted := adaptiveConvergenceTimeout(timeout, snap.MaxReleaseTimeout(), workloadGrace); adapted > timeout {
					deadline = start.Add(adapted)
					util.Msgf("Convergence budget extended to %s to fit slowest HelmRelease timeout (%s)",
						adapted, snap.MaxReleaseTimeout())
				}
			}

			if snap.HelmReleasesReady == snap.HelmReleasesTotal {
				// Every release is Ready. Confirm the resulting workloads have
				// actually stabilized before declaring success, so a release
				// that installs cleanly but crashloops (RBAC/image skew) is
				// caught instead of slipping through.
				if workloadGrace == 0 || len(snap.UnhealthyPods) == 0 {
					util.Msgf("All %d HelmReleases are Ready", snap.HelmReleasesTotal)
					return nil
				}

				if releasesReadyAt.IsZero() {
					releasesReadyAt = time.Now()
					util.Msgf("All %d HelmReleases are Ready; waiting up to %s for %d workload(s) to stabilize",
						snap.HelmReleasesTotal, workloadGrace, len(snap.UnhealthyPods))
				}

				// Track which pods stay unhealthy across consecutive polls.
				current := map[string]bool{}
				for _, pod := range snap.UnhealthyPods {
					current[pod] = true
					unhealthyStreak[pod]++
				}
				for pod := range unhealthyStreak {
					if !current[pod] {
						delete(unhealthyStreak, pod)
					}
				}

				if time.Since(releasesReadyAt) >= workloadGrace {
					var persistent []string
					for pod, streak := range unhealthyStreak {
						if streak >= unhealthyThreshold {
							persistent = append(persistent, pod)
						}
					}
					if len(persistent) > 0 {
						slices.Sort(persistent)
						return fmt.Errorf("install did not converge: all HelmReleases Ready but %d workload(s) unhealthy after %s: %s",
							len(persistent), workloadGrace, strings.Join(persistent, ", "))
					}
					// Grace elapsed with nothing persistently unhealthy — the
					// snapshot's unhealthy pods were transient churn. Accept.
					util.Msgf("All %d HelmReleases are Ready; workloads stabilized", snap.HelmReleasesTotal)
					return nil
				}

				util.Msgf("Convergence: %s", snap.Summary())
			} else {
				util.Msgf("Convergence: %s", snap.Summary())

				// Fail fast on releases Flux has marked Stalled (retries/remediation
				// exhausted) once the signal persists across consecutive polls.
				stalled := snap.StalledReleases()
				current := map[string]bool{}
				for _, r := range stalled {
					current[r.ID()] = true
					stalledStreak[r.ID()]++
				}
				for id := range stalledStreak {
					if !current[id] {
						delete(stalledStreak, id)
					}
				}
				var stuck []provider.HelmReleaseStatus
				for _, r := range stalled {
					if stalledStreak[r.ID()] >= stalledThreshold {
						stuck = append(stuck, r)
					}
				}
				if len(stuck) > 0 {
					return fmt.Errorf("install did not converge: %d HelmRelease(s) stalled (Flux exhausted retries): %s",
						len(stuck), formatReleaseFailures(stuck))
				}

				// Best-effort self-heal: clear Helm release secrets wedged in a
				// pending state, which otherwise block Flux from retrying. Only
				// records stuck past the grace period are scrubbed so we never
				// delete a revision secret helm-controller is actively driving
				// (doing so wedges it forever on "secrets ...vN not found").
				if scrubbed, scrubErr := kube.ScrubStuckHelmReleaseSecrets(ctx, provider.HelmReleaseStuckGracePeriod); scrubErr == nil && len(scrubbed) > 0 {
					util.Msgf("Cleared %d stuck Helm release secret(s) to unblock reconciliation: %v", len(scrubbed), scrubbed)
				}
			}
		}

		if time.Now().After(deadline) {
			elapsed := time.Since(start).Round(time.Second)
			notReady := last.NotReadyDetails()

			// Distinguish a genuinely stuck install (a release Flux has Stalled —
			// retries/remediation exhausted, won't recover without intervention)
			// from one that is merely still progressing. The latter routinely
			// self-heals: Flux keeps reconciling in-cluster after the CLI exits,
			// so a resume that skips completed stages and re-checks convergence
			// usually finds the cluster green. Wording the two cases differently
			// stops a slow-but-healthy install from looking like a hard failure.
			var stalledNow []provider.HelmReleaseStatus
			for _, r := range notReady {
				if r.Stalled {
					stalledNow = append(stalledNow, r)
				}
			}
			if len(stalledNow) > 0 {
				return fmt.Errorf("install did not converge within %s: %d HelmRelease(s) stalled (Flux exhausted retries): %s",
					elapsed, len(stalledNow), formatReleaseFailures(stalledNow))
			}
			return fmt.Errorf("install did not converge within %s: %d/%d HelmReleases ready, none stalled — still progressing. "+
				"Flux keeps reconciling in-cluster; re-run `quartz install` to resume (completed stages are skipped) and confirm convergence. Still not ready: %s",
				elapsed, last.HelmReleasesReady, last.HelmReleasesTotal,
				formatReleaseFailures(notReady))
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
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
			return cloudPreflightError(cp, row.Error)
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

func cloudPreflightError(cp provider.CloudProviderClient, err error) error {
	if cp != nil && strings.EqualFold(cp.ProviderName(), provider.AWS_PROVIDER) {
		return fmt.Errorf(
			"preflight: AWS access check failed: %w\n\n"+
				"AWS credentials are not available or not authorized in this shell. "+
				"Load your shell environment (for example, `source ~/.bashrc`) or export "+
				"AWS_PROFILE/AWS_REGION/AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY/AWS_SESSION_TOKEN, then rerun `quartz install`.",
			err,
		)
	}
	return fmt.Errorf("preflight: cloud access check failed: %w", err)
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

	// No-op fast path: a successful clean destroys the state backend last, so
	// its absence means the environment is already torn down. Probing it once
	// up front lets us skip the parallel init/refresh and per-stage destroy
	// waves entirely — work that would otherwise spend minutes downloading
	// modules and refreshing empty state across every stage only to find
	// nothing to do. On any probe error we fall through to the normal path
	// rather than risk skipping a real teardown.
	if exists, err := TfStateBackendExists(ctx, p); err != nil {
		log.Debug("State backend existence check failed, proceeding with full clean", "error", err)
	} else if !exists {
		util.Msg("State backend not found — environment already torn down, nothing to destroy.")
		if cleanupErr := Cleanup(ctx, p); cleanupErr != nil {
			log.Warn("Final cleanup failed (non-fatal)", "error", cleanupErr)
		}
		return nil
	}

	// Initialize and refresh each stage before destruction. Stages are
	// independent for init/refresh (each operates on its own working directory
	// and remote state), so they run concurrently to avoid the serial
	// per-stage module download + refresh that dominated clean startup time.
	// Uses TfRefreshWithUnlock for automatic state lock recovery.
	initStart := time.Now()
	parallelInitRefresh(ctx, stages, p)
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

	totalDuration := time.Since(cleanupStart)
	printCleanupTimingSummary(stageTiming, totalDuration)

	if len(destroyErrors) > 0 {
		printDestroyErrors(destroyErrors)
	}

	// Persist a durable, plaintext teardown report under the run log directory.
	// The console summary above is lost the moment the working directory is
	// removed (and the operator's terminal scrolls away); on a FAILED clean that
	// post-mortem is exactly what is needed to decide the next step. The report
	// carries no secrets — only stage timings and de-duplicated error messages —
	// so it is always safe to write, unlike the raw tofu destroy output.
	if path, err := persistCleanupReport(p, stageTiming, totalDuration, destroyErrors); err != nil {
		log.Warn("Could not persist cleanup report (non-fatal)", "error", err)
	} else if path != "" {
		util.Msgf("Teardown report written to %s", path)
	}

	if len(destroyErrors) > 0 {
		return fmt.Errorf("%d stage(s) failed to destroy cleanly", len(destroyErrors))
	}

	return nil
}

// cleanupReportDir resolves the directory durable run artifacts are written to,
// derived from the configured file-log path (default "log/...") so the teardown
// report lands alongside the tofu logs regardless of whether file logging is
// enabled. Falls back to "log" when no path is configured.
func cleanupReportDir(cfg schema.QuartzConfig) string {
	p := cfg.Log.File.Path
	if p == "" {
		return "log"
	}
	return filepath.Dir(p)
}

// renderCleanupReport builds the plaintext teardown report: a header, the
// per-phase timing table (sorted for deterministic output), and the grouped
// destroy errors. It deliberately mirrors the console summary but emits no
// color codes so the artifact stays grep- and diff-friendly.
func renderCleanupReport(name string, stageTiming map[string]time.Duration, totalDuration time.Duration, errs []error) string {
	var b strings.Builder
	result := "SUCCESS"
	if len(errs) > 0 {
		result = fmt.Sprintf("FAILED (%d stage(s) did not destroy cleanly)", len(errs))
	}

	fmt.Fprintf(&b, "Quartz Teardown Report\n")
	fmt.Fprintf(&b, "Cluster:   %s\n", name)
	fmt.Fprintf(&b, "Completed: %s\n", time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "Result:    %s\n\n", result)

	fmt.Fprintf(&b, "Timing:\n")
	keys := make([]string, 0, len(stageTiming))
	for k := range stageTiming {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "  %-25s %v\n", k+":", stageTiming[k].Round(time.Second))
	}
	fmt.Fprintf(&b, "  %-25s %v\n", "TOTAL:", totalDuration.Round(time.Second))

	if len(errs) > 0 {
		fmt.Fprintf(&b, "\nDestroy Errors:\n")
		order, stagesByMsg := groupStageErrors(errs)
		for _, msg := range order {
			stageList := stagesByMsg[msg]
			switch {
			case len(stageList) > 1:
				fmt.Fprintf(&b, "  x [%s] %s\n", strings.Join(stageList, ", "), msg)
			case len(stageList) == 1:
				fmt.Fprintf(&b, "  x [%s] %s\n", stageList[0], msg)
			default:
				fmt.Fprintf(&b, "  x %s\n", msg)
			}
		}
	}

	fmt.Fprintf(&b, "\nNext Action:\n")
	if len(errs) > 0 {
		fmt.Fprintf(&b, "  State backend: preserved because one or more stages failed.\n")
		fmt.Fprintf(&b, "  Retry:         quartz clean --yes\n")
		fmt.Fprintf(&b, "  Logs:          inspect the neighboring *.tf.log and *.tf.log.*.gz files for the failed stage.\n")
	} else {
		fmt.Fprintf(&b, "  State backend: destroyed after all stages completed.\n")
		fmt.Fprintf(&b, "  Retry:         not needed.\n")
		fmt.Fprintf(&b, "  Logs:          retained only for audit/troubleshooting.\n")
	}

	return b.String()
}

// persistCleanupReport writes the teardown report to a timestamped file in the
// run log directory and returns its path. The filename follows the same
// "<name>.<date>.<kind>.<unix>" convention as the tofu logs so artifacts from a
// single run sort together.
func persistCleanupReport(p *CommandParams, stageTiming map[string]time.Duration, totalDuration time.Duration, errs []error) (string, error) {
	cfg := p.Settings().Config
	dir := cleanupReportDir(cfg)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", err
	}

	now := time.Now()
	name := cfg.Name
	if name == "" {
		name = "quartz"
	}
	file := filepath.Join(dir, fmt.Sprintf("%s.%s.clean.%d.log", name, now.Format("2006-01-02"), now.Unix()))

	report := renderCleanupReport(name, stageTiming, totalDuration, errs)
	if err := os.WriteFile(file, []byte(report), 0o640); err != nil {
		return "", err
	}
	return file, nil
}

// printDestroyErrors renders the destroy-error summary, collapsing identical
// root-cause messages across stages into a single line. During teardown the
// same failure (most often the EKS "No cluster found" 404) commonly surfaces on
// several stages at once; listing it once with the affected stages is far
// easier to read than N verbatim repetitions.
func printDestroyErrors(errs []error) {
	util.Hdr("Destroy Errors")

	order, stagesByMsg := groupStageErrors(errs)
	for _, msg := range order {
		stageList := stagesByMsg[msg]
		if len(stageList) > 1 {
			util.Msgf("  ✗ [%s] %s", strings.Join(stageList, ", "), msg)
		} else if len(stageList) == 1 {
			util.Msgf("  ✗ [%s] %s", stageList[0], msg)
		} else {
			util.Msgf("  ✗ %s", msg)
		}
	}
}

// groupStageErrors collapses a slice of stage destroy errors by their
// underlying message, preserving first-seen order, so identical root causes
// surfacing on several stages are reported once with the affected stage list.
// Shared by the console summary and the persisted report.
func groupStageErrors(errs []error) ([]string, map[string][]string) {
	order := make([]string, 0, len(errs))
	stagesByMsg := make(map[string][]string)
	for _, e := range errs {
		stage, msg := splitStageError(e)
		if _, seen := stagesByMsg[msg]; !seen {
			order = append(order, msg)
		}
		if stage != "" {
			stagesByMsg[msg] = append(stagesByMsg[msg], stage)
		}
	}
	return order, stagesByMsg
}

// splitStageError separates a "stage <id>: <message>" error into its stage id
// and underlying message so identical messages can be grouped. Errors not in
// that form are returned with an empty stage id and the full message.
func splitStageError(e error) (string, string) {
	s := e.Error()
	if rest, ok := strings.CutPrefix(s, "stage "); ok {
		if idx := strings.Index(rest, ": "); idx > 0 {
			return rest[:idx], rest[idx+2:]
		}
	}
	return "", s
}

// printCleanupTimingSummary outputs timing information for each phase of the
// cleanup. Phases are sorted so the summary is deterministic across runs
// (Go map iteration order is otherwise randomized, scrambling the table).
func printCleanupTimingSummary(stageTiming map[string]time.Duration, totalDuration time.Duration) {
	util.Hdr("Cleanup Timing Summary")
	keys := make([]string, 0, len(stageTiming))
	for k := range stageTiming {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, stage := range keys {
		util.Msgf("  %-25s %v", stage+":", stageTiming[stage].Round(time.Second))
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

		// Self-heal orphaned in-cluster state: when the destroy fails because the
		// Kubernetes/Helm provider can't reach the cluster, OR because Helm could
		// not delete its own release record, the backing objects are already gone
		// (they vanished with the cluster / were deprovisioned by the pre-delete
		// hook). If the cluster is confirmed absent, drop just those orphaned
		// in-cluster resources from state and retry so the stage — and ultimately
		// the state backend teardown — can complete. AWS resources are untouched.
		if (isClusterUnreachableError(errStr) || isHelmReleaseRecordError(errStr)) && clusterAbsent(ctx, p) {
			removed, clearErr := clearOrphanedClusterState(ctx, stage, p)
			if clearErr != nil {
				log.Warn("Failed to clear orphaned in-cluster state", "stage", stage, "error", clearErr)
			} else if removed > 0 {
				util.Msgf("Cluster absent — removed %d orphaned in-cluster resource(s) from stage %s state, retrying destroy", removed, stage)
				// Retry immediately; the remaining (cloud) resources can destroy.
				if retryErr := TfDestroy(ctx, stage, p); retryErr == nil {
					return nil
				} else {
					lastErr = retryErr
					errStr = retryErr.Error()
				}
			}
		}

		// Tolerate already-absent git refs on resumed/re-run cleans: a previous
		// clean may have already deleted the apps branch, so GitHub answers the
		// delete with 422 "Reference does not exist". The branch is genuinely
		// gone, so drop the stale github_branch entries from state and retry
		// rather than fail teardown. This is safe — github_branch manages only a
		// git ref (no cloud/cluster infra), so removing it from state never
		// orphans real resources.
		if isGitRefAbsentError(errStr) {
			removed, clearErr := clearAbsentGitRefState(ctx, stage, p)
			if clearErr != nil {
				log.Warn("Failed to clear already-absent git ref state", "stage", stage, "error", clearErr)
			} else if removed > 0 {
				util.Msgf("Git ref already absent — removed %d stale branch resource(s) from stage %s state, retrying destroy", removed, stage)
				if retryErr := TfDestroy(ctx, stage, p); retryErr == nil {
					return nil
				} else {
					lastErr = retryErr
					errStr = retryErr.Error()
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

// isClusterUnreachableError reports whether a destroy/refresh error stems from
// the Kubernetes/Helm providers being unable to reach the cluster's API server.
// This is distinct from a retryable transient (handled by isRetryableDestroyError):
// when the cluster is permanently gone these never succeed on retry, so the
// orphaned in-cluster state must be cleared instead. The patterns cover the
// EKS data-source lookup 404, the kubernetes provider config_path failure, and
// the discovery-client/RESTMapper failures observed during teardown.
func isClusterUnreachableError(errStr string) bool {
	unreachablePatterns := []string{
		"ResourceNotFoundException",
		"No cluster found",
		"couldn't find resource",          // data.aws_eks_cluster lookup miss
		"cannot create discovery client",  // kubernetes provider, no client config
		"Failed to get RESTMapper client", // kubernetes_resource data source
		"config_path",                     // provider "kubernetes" {} invalid kubeconfig path
		"Kubernetes cluster unreachable",
		"the server could not find the requested resource",
	}
	for _, pattern := range unreachablePatterns {
		if len(errStr) > 0 && strings.Contains(errStr, pattern) {
			return true
		}
	}
	return false
}

// isHelmReleaseRecordError reports whether a destroy error is the Helm
// "release record" deletion failure — the pre-delete hook already deprovisioned
// the backing cloud/in-cluster objects, but Helm could not delete its own
// release bookkeeping (e.g. a stuck finalizer on an orphaned CRD). The managed
// infrastructure is already gone, so the remedy is to drop the stuck
// helm_release from state rather than fail the whole teardown.
func isHelmReleaseRecordError(errStr string) bool {
	recordPatterns := []string{
		"failed to delete release",
		"Unable to uninstall Helm release",
	}
	for _, pattern := range recordPatterns {
		if len(errStr) > 0 && strings.Contains(errStr, pattern) {
			return true
		}
	}
	return false
}

// clusterAbsent reports whether the target EKS cluster is confirmed gone. It is
// used to gate destructive state surgery (dropping orphaned in-cluster resources)
// so we only do so when the cluster genuinely no longer exists — never on a
// transient connectivity blip. A nil error (cluster reachable) or any non
// "not found" error returns false.
func clusterAbsent(ctx context.Context, p *CommandParams) bool {
	_, err := p.Provider().Kubernetes(ctx)
	if err == nil {
		return false
	}
	return isClusterNotFoundError(err)
}

// clearOrphanedClusterState drops the in-cluster (Helm/Kubernetes) MANAGED
// resources from a stage's state so a subsequent destroy can complete. It is a
// no-op returning (0, nil) when there is nothing to clear. AWS-provider
// resources in the stage are preserved for normal destruction.
func clearOrphanedClusterState(ctx context.Context, stage string, p *CommandParams) (int, error) {
	client := tofu.Instance(ctx, *p.Settings())
	s := p.Settings().Config.Stages[stage]
	return client.StateRemoveOrphanedClusterResources(ctx, s)
}

// isGitRefAbsentError reports whether a destroy error stems from deleting a git
// ref (branch) that is already gone. GitHub answers a delete of a non-existent
// ref with 422 "Reference does not exist", which surfaces on a resumed or
// repeated clean after the branch was removed by an earlier run.
func isGitRefAbsentError(errStr string) bool {
	return len(errStr) > 0 && strings.Contains(errStr, "Reference does not exist")
}

// clearAbsentGitRefState drops github_branch resources from a stage's state so a
// subsequent destroy can converge when the underlying ref is already gone. It
// is safe because github_branch manages only a git ref — removing it from state
// never orphans cloud or cluster infrastructure. Returns the number of entries
// removed (0 when there is nothing to clear).
func clearAbsentGitRefState(ctx context.Context, stage string, p *CommandParams) (int, error) {
	client := tofu.Instance(ctx, *p.Settings())
	s := p.Settings().Config.Stages[stage]
	addrs, err := client.StateList(ctx, s, "github_branch.")
	if err != nil {
		return 0, err
	}
	if len(addrs) == 0 {
		return 0, nil
	}
	if err := client.StateRemove(ctx, s, addrs...); err != nil {
		return 0, err
	}
	return len(addrs), nil
}

// stageStateEmpty reports whether a stage's state has no resource instances
// recorded (the structured equivalent of an empty `tofu state list`). It is used
// to skip the pre-destroy refresh for stages with nothing to refresh — which
// both saves time and, more importantly, avoids noisy provider-config errors
// (e.g. the kubernetes provider failing on a missing kubeconfig) for stages that
// have no objects to reconcile. On any read error it returns false so the caller
// falls back to the normal refresh path.
func stageStateEmpty(ctx context.Context, stage string, p *CommandParams) bool {
	client := tofu.Instance(ctx, *p.Settings())
	s := p.Settings().Config.Stages[stage]
	addrs, err := client.StateList(ctx, s)
	if err != nil {
		log.Debug("Could not determine if stage state is empty, assuming non-empty", "stage", stage, "error", err)
		return false
	}
	return len(addrs) == 0
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
		// Keycloak provisions realms against a multi-replica StatefulSet behind a
		// Service. While Keycloak is still rolling out (or being rescaled), the
		// parallel realm-create requests load-balance across replicas and one may
		// hit a pod whose theme cache has not finished loading yet, yielding
		// `validation error: theme "quartz" does not exist on the server`. The
		// theme does exist (sibling realms in the same apply succeed); this is an
		// eventual-consistency race that clears once the rollout settles, so retry.
		// Re-apply is idempotent: realms created before the failure are already in
		// state and plan as no-ops.
		"does not exist on the server",
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

// parallelInitRefresh initializes and refreshes every stage concurrently in
// preparation for destroy. Each stage operates on its own working directory and
// remote state, so there is no inter-stage ordering requirement here (unlike
// destroy, which must respect reverse dependencies). Failures are logged but
// not returned: a stage that cannot init or refresh is still attempted during
// the destroy phase, matching the prior serial behavior.
func parallelInitRefresh(ctx context.Context, stages []schema.StageConfig, p *CommandParams) {
	var wg sync.WaitGroup
	for _, s := range stages {
		wg.Add(1)
		go func(stage schema.StageConfig) {
			defer wg.Done()

			if err := TfInit(ctx, stage.Id, p); err != nil {
				log.Warn("Init failed for stage, will attempt destroy anyway", "stage", stage.Id, "error", err)
			}

			// Skip the pre-destroy refresh for stages with empty state: there
			// is nothing to reconcile, and refreshing a service-dependent stage
			// whose cluster is already gone only produces noisy provider-config
			// errors.
			if stageStateEmpty(ctx, stage.Id, p) {
				log.Debug("Stage state empty, skipping pre-destroy refresh", "stage", stage.Id)
				return
			}

			if err := TfRefreshWithUnlock(ctx, stage.Id, p); err != nil {
				// The umbrella Helm release is version-stamped by Flux
				// ("1.0.0+<sha>") once it adopts the bootstrap release, so a
				// pre-destroy refresh of the core stage surfaces the benign Helm
				// provider "Planned version is different from configured
				// version" mismatch. The subsequent destroy runs with
				// Refresh(false) and is unaffected, so this is cosmetic — demote
				// it to debug to avoid alarming clean output.
				if isFluxOwnedReleaseDrift(err) {
					log.Debug("Refresh reported benign Flux-owned release version drift, continuing", "stage", stage.Id, "error", err)
				} else {
					log.Warn("Refresh failed for stage", "stage", stage.Id, "error", err)
				}
			}
		}(s)
	}
	wg.Wait()
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
