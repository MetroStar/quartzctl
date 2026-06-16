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
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MetroStar/quartzctl/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"
)

func TestNewRootInstallCommand(t *testing.T) {
	p := defaultTestConfig(t)
	cmd := NewRootInstallCommand(p).Command

	assert.Equal(t, "install", cmd.Name)
	assert.Equal(t, "Perform a full install/update of the system", cmd.Usage)
	assert.Len(t, cmd.Flags, 4)

	waitFlag := cmd.Flags[2].(*cli.BoolFlag)
	assert.Equal(t, "wait-for-models", waitFlag.Name)

	err := cmd.Action(context.Background(), &cli.Command{})
	assert.NoError(t, err)
}

func TestNewRootCleanCommand(t *testing.T) {
	p := defaultTestConfig(t)
	cmd := NewRootCleanCommand(p).Command

	assert.Equal(t, "clean", cmd.Name)
	assert.Equal(t, "Perform a full cleanup/teardown of the system", cmd.Usage)
	assert.Len(t, cmd.Flags, 2)

	flag := cmd.Flags[0].(*cli.BoolFlag)
	assert.Equal(t, "refresh", flag.Name)

	yesFlag := cmd.Flags[1].(*cli.BoolFlag)
	assert.Equal(t, "yes", yesFlag.Name)

	err := cmd.Action(context.Background(), &cli.Command{})
	assert.NoError(t, err)
}

func TestCmdInstall(t *testing.T) {
	p := defaultTestConfig(t)

	err := Install(context.Background(), p, "")
	if err != nil {
		t.Errorf("unexpected error in cmd Install, %v", err)
	}
}

func TestCmdClean(t *testing.T) {
	p := defaultTestConfig(t)

	err := Clean(context.Background(), p)
	if err != nil {
		t.Errorf("unexpected error in cmd Clean, %v", err)
	}
}

// backendGoneCloud is a cloud provider that reports its state backend as already
// destroyed, exercising the no-op clean fast path. It embeds LocalClient so it
// satisfies the full CloudProviderClient interface with only the existence probe
// overridden.
type backendGoneCloud struct {
	provider.LocalClient
}

func (backendGoneCloud) StateBackendExists(context.Context) (bool, error) {
	return false, nil
}

type failingAWSPreflightCloud struct {
	provider.LocalClient
	err error
}

func (c failingAWSPreflightCloud) ProviderName() string {
	return provider.AWS_PROVIDER
}

func (c failingAWSPreflightCloud) CheckAccess(context.Context) provider.ProviderCheckResult {
	return provider.AwsProviderCheckResult{Error: c.err}
}

func TestCmdCleanNoOpFastPath(t *testing.T) {
	p := defaultTestConfig(t)
	k8s, err := p.Provider().Kubernetes(context.Background())
	if err != nil {
		t.Fatalf("unexpected error getting kubernetes provider, %v", err)
	}

	// Swap in a cloud provider whose state backend is already gone so the clean
	// short-circuits before any init/refresh/destroy work.
	p.provider = provider.NewProviderFactory(
		p.Settings().Config,
		p.Settings().Secrets,
		provider.WithKubernetesProvider(k8s),
		provider.WithCloudProvider(backendGoneCloud{provider.LocalClient{Name: "test"}}),
	)

	err = Clean(context.Background(), p)
	if err != nil {
		t.Errorf("unexpected error in no-op clean fast path, %v", err)
	}
}

func TestPreflightAwsMissingShellEnvMessage(t *testing.T) {
	p := defaultTestConfig(t)
	k8s, err := p.Provider().Kubernetes(context.Background())
	if err != nil {
		t.Fatalf("unexpected error getting kubernetes provider, %v", err)
	}

	p.provider = provider.NewProviderFactory(
		p.Settings().Config,
		p.Settings().Secrets,
		provider.WithKubernetesProvider(k8s),
		provider.WithCloudProvider(failingAWSPreflightCloud{
			LocalClient: provider.LocalClient{Name: "test"},
			err:         errors.New("failed to refresh cached credentials"),
		}),
	)

	err = Preflight(context.Background(), p)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "AWS access check failed")
	assert.Contains(t, err.Error(), "source ~/.bashrc")
}

func TestLocalInstallPreflightCompressesOldTofuLogs(t *testing.T) {
	p := defaultTestConfig(t)
	logDir := filepath.Join(t.TempDir(), "log")
	assert.NoError(t, os.MkdirAll(logDir, 0755))

	oldLog := filepath.Join(logDir, "old.tf.log")
	assert.NoError(t, os.WriteFile(oldLog, []byte("quartz-log\n"), 0644))
	assert.NoError(t, os.Truncate(oldLog, logPreflightCompressMinBytes+1))
	oldTime := time.Now().Add(-2 * logPreflightCompressMinAge)
	assert.NoError(t, os.Chtimes(oldLog, oldTime, oldTime))

	p.Settings().Config.Log.Tofu.Path = filepath.Join(logDir, "$name.$date.tf.log")
	p.Settings().Config.Log.File.Path = filepath.Join(logDir, "$name.$date.log")

	assert.NoError(t, LocalInstallPreflight(p))
	assert.NoFileExists(t, oldLog)

	gzPath := oldLog + ".gz"
	assert.FileExists(t, gzPath)
	f, err := os.Open(gzPath)
	assert.NoError(t, err)
	defer f.Close()
	zr, err := gzip.NewReader(f)
	assert.NoError(t, err)
	defer zr.Close()
	prefix := make([]byte, len("quartz-log\n"))
	_, err = io.ReadFull(zr, prefix)
	assert.NoError(t, err)
	assert.Equal(t, "quartz-log\n", string(prefix))
}

func TestIsRetryableDestroyError(t *testing.T) {
	tests := []struct {
		name     string
		errStr   string
		expected bool
	}{
		{
			name:     "DependencyViolation error",
			errStr:   "Error: DependencyViolation: resource has dependencies",
			expected: true,
		},
		{
			name:     "has a dependent object error",
			errStr:   "cannot delete: has a dependent object",
			expected: true,
		},
		{
			name:     "NetworkInterfaceInUse error",
			errStr:   "Error: NetworkInterfaceInUse: interface eni-123 is in use",
			expected: true,
		},
		{
			name:     "InvalidGroup.InUse error",
			errStr:   "Error: InvalidGroup.InUse: security group sg-123 is in use",
			expected: true,
		},
		{
			name:     "Helm failed to delete release",
			errStr:   "Error: failed to delete release: connection refused",
			expected: true,
		},
		{
			name:     "Kubernetes cluster unreachable",
			errStr:   "Kubernetes cluster unreachable: dial tcp timeout",
			expected: true,
		},
		{
			name:     "connection refused",
			errStr:   "Post https://api.cluster.local: connection refused",
			expected: true,
		},
		{
			name:     "no endpoints available",
			errStr:   "Internal error: failed calling webhook: no endpoints available",
			expected: true,
		},
		{
			name:     "i/o timeout",
			errStr:   "Post https://api.cluster.local: i/o timeout",
			expected: true,
		},
		{
			name:     "unrelated error",
			errStr:   "Error: resource not found",
			expected: false,
		},
		{
			name:     "empty string",
			errStr:   "",
			expected: false,
		},
		{
			name:     "permission denied",
			errStr:   "Error: Access Denied",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryableDestroyError(tt.errStr)
			assert.Equal(t, tt.expected, result, "isRetryableDestroyError(%q) = %v, want %v", tt.errStr, result, tt.expected)
		})
	}
}

func TestIsClusterUnreachableError(t *testing.T) {
	tests := []struct {
		name     string
		errStr   string
		expected bool
	}{
		{name: "resource not found", errStr: "Error: ResourceNotFoundException: No cluster found", expected: true},
		{name: "no cluster found", errStr: "No cluster found for name pa-1", expected: true},
		{name: "discovery client", errStr: "cannot create discovery client", expected: true},
		{name: "rest mapper", errStr: "Failed to get RESTMapper client", expected: true},
		{name: "config path", errStr: "provider config_path is set but file is missing", expected: true},
		{name: "cluster unreachable", errStr: "Kubernetes cluster unreachable: timeout", expected: true},
		{name: "could not find resource", errStr: "the server could not find the requested resource", expected: true},
		{name: "unrelated", errStr: "DependencyViolation: in use", expected: false},
		{name: "empty", errStr: "", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isClusterUnreachableError(tt.errStr))
		})
	}
}

func TestIsHelmReleaseRecordError(t *testing.T) {
	tests := []struct {
		name     string
		errStr   string
		expected bool
	}{
		{name: "failed to delete release", errStr: "Error: failed to delete release: cilium", expected: true},
		{name: "unable to uninstall", errStr: "Unable to uninstall Helm release foo", expected: true},
		{name: "unrelated", errStr: "DependencyViolation: in use", expected: false},
		{name: "empty", errStr: "", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isHelmReleaseRecordError(tt.errStr))
		})
	}
}

func TestSplitStageError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantStage string
		wantMsg   string
	}{
		{name: "stage prefixed", err: fmt.Errorf("stage prereqs: No cluster found"), wantStage: "prereqs", wantMsg: "No cluster found"},
		{name: "no stage prefix", err: fmt.Errorf("cleanup: boom"), wantStage: "", wantMsg: "cleanup: boom"},
		{name: "stage word but no colon", err: fmt.Errorf("stage failure"), wantStage: "", wantMsg: "stage failure"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stage, msg := splitStageError(tt.err)
			assert.Equal(t, tt.wantStage, stage)
			assert.Equal(t, tt.wantMsg, msg)
		})
	}
}

func TestRenderCleanupReportSuccess(t *testing.T) {
	timing := map[string]time.Duration{
		"init-refresh":    12 * time.Second,
		"destroy-network": 90 * time.Second,
	}
	out := renderCleanupReport("pa-test", timing, 2*time.Minute, nil, nil)

	assert.Contains(t, out, "Quartz Teardown Report")
	assert.Contains(t, out, "Cluster:   pa-test")
	assert.Contains(t, out, "Result:    SUCCESS")
	assert.Contains(t, out, "TOTAL:")
	// Sorted, deterministic order: destroy-network precedes init-refresh.
	assert.Less(t, strings.Index(out, "destroy-network"), strings.Index(out, "init-refresh"))
	assert.Contains(t, out, "Next Action:")
	assert.Contains(t, out, "State backend: destroyed")
	assert.Contains(t, out, "Provider wait:")
	// No error section on a clean teardown.
	assert.NotContains(t, out, "Destroy Errors:")
}

func TestRenderCleanupReportWithErrors(t *testing.T) {
	timing := map[string]time.Duration{"destroy-cluster": 30 * time.Second}
	errs := []error{
		fmt.Errorf("stage cluster: No cluster found"),
		fmt.Errorf("stage network: No cluster found"),
		fmt.Errorf("stage host: boom"),
	}
	out := renderCleanupReport("pa-test", timing, time.Minute, errs, nil)

	assert.Contains(t, out, "Result:    FAILED (3 stage(s) did not destroy cleanly)")
	assert.Contains(t, out, "Destroy Errors:")
	// Identical messages are grouped onto a single line with both stages.
	assert.Contains(t, out, "[cluster, network] No cluster found")
	assert.Contains(t, out, "[host] boom")
	assert.Contains(t, out, "Next Action:")
	assert.Contains(t, out, "State backend: preserved")
	assert.Contains(t, out, "quartz clean --yes")
}

func TestRenderCleanupReportWithCleanupStatus(t *testing.T) {
	timing := map[string]time.Duration{"destroy-core": 2 * time.Second}
	status := &provider.CleanupStatus{
		Data: map[string]string{
			"status":         "Succeeded",
			"phase":          "complete",
			"detail":         "Pre-delete hook completed",
			"updatedAt":      "2026-06-11T12:00:00Z",
			"degraded":       "true",
			"degradedDetail": "Karpenter finalizer lag detected",
		},
		Events: []provider.CleanupEvent{{
			Reason:        "KarpenterFinalizerLag",
			Type:          "Warning",
			Message:       "All remaining NodeClaims report drain, volume detach, and instance termination requested.",
			LastTimestamp: time.Date(2026, 6, 11, 12, 0, 1, 0, time.UTC),
		}},
		HookEvents: []provider.CleanupHookEvent{
			{
				At:     time.Date(2026, 6, 11, 11, 59, 58, 0, time.UTC),
				Kind:   "status",
				Phase:  "flux",
				Status: "Running",
				Detail: "Suspending Flux reconciliation",
			},
			{
				At:     time.Date(2026, 6, 11, 12, 0, 0, 0, time.UTC),
				Kind:   "degraded",
				Phase:  "nodeclaims",
				Status: "Degraded",
				Detail: "Karpenter finalizer lag detected",
			},
		},
	}

	out := renderCleanupReport("pa-test", timing, time.Minute, nil, status)

	assert.Contains(t, out, "Cleanup Hook:")
	assert.Contains(t, out, "Status:      Succeeded")
	assert.Contains(t, out, "Degraded:    Karpenter finalizer lag detected")
	assert.Contains(t, out, "Recovery:    self-healed after Karpenter finalizer lag detected; cleanup hook finished successfully")
	assert.Contains(t, out, "Hook History: 2 recorded step(s), 1 degraded")
	assert.Contains(t, out, "nodeclaims")
	assert.Contains(t, out, "KarpenterFinalizerLag")
	assert.Contains(t, out, "Notes:")
	assert.Contains(t, out, "No manual action is required for that hook condition")
}

func TestCleanupNotesDescribeLongCleanPhases(t *testing.T) {
	notes := cleanupNotes(map[string]time.Duration{
		"init-refresh": 3 * time.Minute,
		"destroy-core": 6 * time.Minute,
		"destroy-host": 18 * time.Minute,
	}, nil, nil)

	joined := strings.Join(notes, "\n")
	assert.Contains(t, joined, "init-refresh runs stages in parallel")
	assert.Contains(t, joined, "destroy-core includes the Helm pre-delete hook")
	assert.Contains(t, joined, "destroy-host includes provider-side managed-service teardown")
}

func TestModelListContainsAll(t *testing.T) {
	assert.True(t, modelListContainsAll("gemma4:e4b,gemma4:12b", "gemma4:12b, gemma4:e4b"))
	assert.True(t, modelListContainsAll("", ""))
	assert.False(t, modelListContainsAll("gemma4:e4b,gemma4:12b", "gemma4:e4b"))
}

func TestInstallWaitForModels(t *testing.T) {
	p := defaultTestConfig(t)
	t.Setenv("QUARTZ_WAIT_FOR_MODELS", "")
	assert.False(t, installWaitForModels(p))

	t.Setenv("QUARTZ_WAIT_FOR_MODELS", "true")
	assert.True(t, installWaitForModels(p))

	t.Setenv("QUARTZ_WAIT_FOR_MODELS", "false")
	p.waitForModels = true
	assert.True(t, installWaitForModels(p))
}

func TestModelWarmerWaitTimeout(t *testing.T) {
	t.Setenv("QUARTZ_MODEL_WARMER_TIMEOUT", "")
	assert.Equal(t, 90*time.Minute, modelWarmerWaitTimeout())

	t.Setenv("QUARTZ_MODEL_WARMER_TIMEOUT", "2h")
	assert.Equal(t, 2*time.Hour, modelWarmerWaitTimeout())

	t.Setenv("QUARTZ_MODEL_WARMER_TIMEOUT", "0")
	assert.Equal(t, time.Duration(0), modelWarmerWaitTimeout())
}

func TestSanitizeModelWarmerStatusJSON(t *testing.T) {
	raw := "\x1b[0m\n {\"phase\":\"Running\",\"step\":\"pull\",\"detail\":\"downloading\"}\x00"
	assert.Equal(t,
		"{\"phase\":\"Running\",\"step\":\"pull\",\"detail\":\"downloading\"}",
		sanitizeModelWarmerStatusJSON(raw),
	)
}

func TestModelWarmerCompletionTime(t *testing.T) {
	assert.Equal(t,
		"2026-06-16T12:05:00Z",
		modelWarmerCompletionTime(modelWarmerStatus{
			CompletedAt: "2026-06-16T12:05:00Z",
			UpdatedAt:   "2026-06-16T12:04:00Z",
		}),
	)
	assert.Equal(t,
		"2026-06-16T12:04:00Z",
		modelWarmerCompletionTime(modelWarmerStatus{
			UpdatedAt: "2026-06-16T12:04:00Z",
		}),
	)
	assert.Equal(t, "", modelWarmerCompletionTime(modelWarmerStatus{}))
}

func TestDestroyStagePreamble(t *testing.T) {
	assert.Contains(t, destroyStagePreamble("host"), "provider-managed services")
	assert.Equal(t, "", destroyStagePreamble("core"))
}

func TestReadModelWarmerStatusSanitizesControlCharacters(t *testing.T) {
	p := defaultTestConfig(t)
	kube, err := p.Provider().Kubernetes(context.Background())
	assert.NoError(t, err)

	status, ok, err := readModelWarmerStatus(context.Background(), kube)
	assert.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, "Running", status.Phase)
	assert.Equal(t, "pull", status.Step)
	assert.Equal(t, "gemma4:12b", status.Model)
	assert.Equal(t, "downloading", status.Detail)
}

func TestSlowestTimingEntries(t *testing.T) {
	entries := slowestTimingEntries(map[string]time.Duration{
		"stage-a": 10 * time.Second,
		"stage-b": time.Minute,
		"stage-c": time.Minute,
	}, 2)

	assert.Len(t, entries, 2)
	assert.Equal(t, "stage-b", entries[0].Name)
	assert.Equal(t, "stage-c", entries[1].Name)
}

func TestPersistCleanupReport(t *testing.T) {
	dir := t.TempDir()
	p := defaultTestConfig(t)
	p.Settings().Config.Name = "pa-test"
	p.Settings().Config.Log.File.Path = filepath.Join(dir, "$name.$date.log")

	timing := map[string]time.Duration{"cleanup-final": time.Second}
	path, err := persistCleanupReport(p, timing, time.Second, nil, nil)
	assert.NoError(t, err)
	assert.FileExists(t, path)

	data, err := os.ReadFile(path) // #nosec G304
	assert.NoError(t, err)
	assert.Contains(t, string(data), "Quartz Teardown Report")
	assert.Contains(t, string(data), "Result:    SUCCESS")
}

func TestIsFluxOwnedReleaseDrift(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name: "helm provider version mismatch (Flux-owned bootstrap release)",
			err: errors.New(`Error: Planned version is different from configured version` +
				"\n  with helm_release.quartz,\n" +
				`The version in the configuration is "1.0.0+df9c1f7b78c1" but the planned version is "1.0.0".`),
			expected: true,
		},
		{
			name:     "unrelated plan error",
			err:      errors.New("Error: Backend initialization required"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isFluxOwnedReleaseDrift(tt.err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHelmConvergenceTimeout(t *testing.T) {
	t.Setenv("QUARTZ_CONVERGENCE_TIMEOUT", "")
	d, explicit := helmConvergenceTimeout()
	assert.Equal(t, 30*time.Minute, d)
	assert.False(t, explicit, "unset env should not be explicit")

	t.Setenv("QUARTZ_CONVERGENCE_TIMEOUT", "45m")
	d, explicit = helmConvergenceTimeout()
	assert.Equal(t, 45*time.Minute, d)
	assert.True(t, explicit, "set env should be explicit")

	// Garbage falls back to the (non-explicit) default.
	t.Setenv("QUARTZ_CONVERGENCE_TIMEOUT", "not-a-duration")
	d, explicit = helmConvergenceTimeout()
	assert.Equal(t, 30*time.Minute, d)
	assert.False(t, explicit)
}

func TestAdaptiveConvergenceTimeout(t *testing.T) {
	floor := 30 * time.Minute
	grace := 3 * time.Minute

	// No release declares a timeout: keep the floor.
	assert.Equal(t, floor, adaptiveConvergenceTimeout(floor, 0, grace))

	// Slow release (90m, e.g. ollama cold pull): 2*90m + 3m, well above floor.
	assert.Equal(t, 183*time.Minute, adaptiveConvergenceTimeout(floor, 90*time.Minute, grace))

	// A release whose timeout is small enough that 2*T+grace stays under the
	// floor must not shrink the budget below the floor.
	assert.Equal(t, floor, adaptiveConvergenceTimeout(floor, 5*time.Minute, grace))
}

func TestClusterProgressMaxReleaseTimeout(t *testing.T) {
	p := provider.ClusterProgress{
		Releases: []provider.HelmReleaseStatus{
			{Namespace: "quartz", Name: "kiali", Timeout: 45 * time.Minute},
			{Namespace: "quartz", Name: "ollama", Timeout: 90 * time.Minute},
			{Namespace: "quartz", Name: "reloader"}, // no timeout
		},
	}
	assert.Equal(t, 90*time.Minute, p.MaxReleaseTimeout())

	// Empty snapshot reports zero (gate then keeps its floor).
	assert.Equal(t, time.Duration(0), provider.ClusterProgress{}.MaxReleaseTimeout())
}
