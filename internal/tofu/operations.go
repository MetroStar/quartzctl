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

package tofu

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/MetroStar/quartzctl/internal/config/schema"
	"github.com/MetroStar/quartzctl/internal/log"
	"github.com/MetroStar/quartzctl/internal/util"

	"github.com/hashicorp/terraform-exec/tfexec"
	tfjson "github.com/hashicorp/terraform-json"

	"github.com/tidwall/gjson"
)

// sensitiveVarNamePattern matches OpenTofu variable names that are likely to
// carry secret material. It is used to redact values from debug logs even when
// the value did not arrive via the explicitly-secret (`v.Secret`) path — e.g. a
// token sourced from an env var or a prior stage's output. Anchored on common
// secret-bearing substrings so it errs toward redaction.
var sensitiveVarNamePattern = regexp.MustCompile(`(?i)(token|password|passwd|secret|private_key|client_secret|credential|api[_-]?key|access[_-]?key)`)

// redactSensitiveVar returns "[REDACTED]" when the variable name suggests the
// value is a secret, otherwise the value unchanged. This keeps debug logs
// useful for non-sensitive plumbing while never spilling credentials.
func redactSensitiveVar(name string, val string) string {
	if val != "" && sensitiveVarNamePattern.MatchString(name) {
		return "[REDACTED]"
	}
	return val
}

// Version retrieves the version of the OpenTofu CLI.
// It runs the `tofu version` command and returns the version string.
func (c *TofuClient) Version(ctx context.Context) (string, error) {
	log.Debug("tofu version")

	tf, err := c.newTfOpts(&TfOpts{})
	if err != nil {
		return "", err
	}

	tfVersion, _, err := tf.Version(ctx, true)
	if err != nil {
		return "", err
	}

	return tfVersion.String(), nil
}

// Init initializes the OpenTofu working directory for the specified stage.
// It runs `tofu init -upgrade -reconfigure` with the provided backend configuration options.
func (c *TofuClient) Init(ctx context.Context, stage schema.StageConfig, opts TofuInitOpts) error {
	log.Debug("tofu init", "stage", stage)

	var args []tfexec.InitOption
	args = append(args, tfexec.Upgrade(true))
	args = append(args, tfexec.Reconfigure(true))
	for _, bc := range opts.BackendConfig {
		args = append(args, tfexec.BackendConfig(bc))
	}

	tf, err := c.getTf(stage.Path)
	if err != nil {
		return err
	}
	c.setStageEnv(tf, stage)
	return tf.Init(ctx, args...)
}

// Validate validates the OpenTofu configuration for the specified stage.
// It runs `tofu validate` and returns the validation output.
func (c *TofuClient) Validate(ctx context.Context, stage schema.StageConfig) (*tfjson.ValidateOutput, error) {
	log.Debug("tofu validate", "stage", stage)
	tf, err := c.getTf(stage.Path)
	if err != nil {
		return nil, err
	}
	return tf.Validate(ctx)
}

// Format formats the OpenTofu configuration files in the specified stage directory.
// It runs `tofu fmt -recursive`.
func (c *TofuClient) Format(ctx context.Context, stage schema.StageConfig) error {
	log.Debug("tofu fmt", "stage", stage)
	tf, err := c.getTf(stage.Path)
	if err != nil {
		return err
	}
	return tf.FormatWrite(ctx, tfexec.Recursive(true))
}

// Plan creates an execution plan for the specified stage.
// It runs `tofu plan` with the configured input variables and returns whether changes are required.
func (c *TofuClient) Plan(ctx context.Context, stage schema.StageConfig) (bool, error) {
	log.Debug("tofu plan", "stage", stage)
	tf, err := c.getTf(stage.Path)
	if err != nil {
		return false, err
	}

	stageVarFile, cleanup, err := c.stageVarFile(ctx, stage)
	if err != nil {
		return false, err
	}
	defer cleanup()

	var vars []tfexec.PlanOption
	if !stage.OverrideVars {
		vars = append(vars, tfexec.VarFile(c.cfg.Config.TfVarFilePath()))
	}
	if stageVarFile != "" {
		vars = append(vars, tfexec.VarFile(stageVarFile))
	}
	c.setStageEnv(tf, stage)
	return tf.Plan(ctx, vars...)
}

// TofuApplyOpts contains options for Apply operations.
type TofuApplyOpts struct {
	AllowDeferral bool     // Enable OpenTofu deferred actions (-allow-deferral)
	Targets       []string // Restrict the apply to these resource addresses (-target)
}

// Apply applies the OpenTofu configuration for the specified stage.
// It runs `tofu apply` with the configured input variables.
func (c *TofuClient) Apply(ctx context.Context, stage schema.StageConfig, opts ...TofuApplyOpts) error {
	if stage.Debug.Break {
		util.Msgf("Break point at stage %s", stage.Id)
		return fmt.Errorf("break")
	}

	log.Debug("tofu apply", "stage", stage)
	tf, err := c.getTf(stage.Path)
	if err != nil {
		return err
	}

	stageVarFile, cleanup, err := c.stageVarFile(ctx, stage)
	if err != nil {
		return err
	}
	defer cleanup()

	var vars []tfexec.ApplyOption
	if !stage.OverrideVars {
		vars = append(vars, tfexec.VarFile(c.cfg.Config.TfVarFilePath()))
	}
	if stageVarFile != "" {
		vars = append(vars, tfexec.VarFile(stageVarFile))
	}

	// Apply deferred actions flag if requested
	if len(opts) > 0 && opts[0].AllowDeferral {
		vars = append(vars, tfexec.AllowDeferral(true))
	}

	// Restrict the apply to specific resource addresses when requested. Used to
	// converge a stage's co-resources (e.g. the values overlay Secret) without
	// touching a Flux-adopted Helm release, whose version drift would otherwise
	// abort an untargeted plan before any resource applies.
	if len(opts) > 0 {
		for _, t := range opts[0].Targets {
			vars = append(vars, tfexec.Target(t))
		}
	}

	c.setStageEnv(tf, stage)

	return tf.Apply(ctx, vars...)
}

// Destroy destroys the OpenTofu-managed infrastructure for the specified stage.
// It runs `tofu destroy` with the configured input variables and targets.
func (c *TofuClient) Destroy(ctx context.Context, stage schema.StageConfig) error {
	if stage.Debug.Break {
		util.Msgf("Break point at stage %s", stage.Id)
		return fmt.Errorf("break")
	}

	if stage.Destroy.Skip {
		util.Msgf("Destruction skipped for stage %s", stage.Id)
		return nil
	}

	log.Debug("tofu destroy", "stage", stage)
	tf, err := c.getTf(stage.Path)
	if err != nil {
		return err
	}

	stageVarFile, cleanup, err := c.stageVarFile(ctx, stage)
	if err != nil {
		return err
	}
	defer cleanup()

	var vars []tfexec.DestroyOption
	vars = append(vars, tfexec.Refresh(false))
	// Short lock timeout: fail fast on stale locks so retry logic can force-unlock
	vars = append(vars, tfexec.LockTimeout("10s"))
	if !stage.OverrideVars {
		vars = append(vars, tfexec.VarFile(c.cfg.Config.TfVarFilePath()))
	}
	if stageVarFile != "" {
		vars = append(vars, tfexec.VarFile(stageVarFile))
	}

	targets, found, err := targetsToDestroy(ctx, c, stage)
	if err != nil {
		return err
	}

	if !found {
		log.Info("No matching state entries found, bypassing destroy", "stage", stage.Id)
		return nil
	}

	for _, t := range targets {
		log.Warn("Explicitly destroying", "resource", t, "stage", stage.Id)
		vars = append(vars, tfexec.Target(t))
	}

	c.setStageEnv(tf, stage)
	return tf.Destroy(ctx, vars...)
}

// Refresh updates the OpenTofu state for the specified stage.
// It runs `tofu refresh` with the configured input variables.
func (c *TofuClient) Refresh(ctx context.Context, stage schema.StageConfig) error {
	log.Debug("tofu refresh", "stage", stage)

	// Route refresh output through a filter that suppresses benign diagnostic
	// blocks emitted while tearing an environment down (e.g. Flux-owned Helm
	// release version drift, optional S3 sub-resources that don't exist). The
	// destroy that follows runs with refresh disabled and is unaffected, so this
	// only spares the operator alarming-but-harmless noise. A fresh, non-cached
	// instance is used (like getTfQuiet) so the wrapped writers are stage-local.
	stdout := newNoiseFilterWriter(os.Stdout)
	stderr := newNoiseFilterWriter(os.Stderr)
	tf, err := c.newTfOpts(&TfOpts{dir: stage.Path, stdout: stdout, stderr: stderr})
	if err != nil {
		return err
	}
	c.setStageEnv(tf, stage)

	stageVarFile, cleanup, err := c.stageVarFile(ctx, stage)
	if err != nil {
		return err
	}
	defer cleanup()

	var vars []tfexec.RefreshCmdOption
	if !stage.OverrideVars {
		vars = append(vars, tfexec.VarFile(c.cfg.Config.TfVarFilePath()))
	}
	if stageVarFile != "" {
		vars = append(vars, tfexec.VarFile(stageVarFile))
	}

	refreshErr := tf.Refresh(ctx, vars...)
	if flushErr := stdout.Flush(); flushErr != nil {
		log.Debug("Failed flushing refresh stdout filter", "stage", stage.Id, "error", flushErr)
	}
	if flushErr := stderr.Flush(); flushErr != nil {
		log.Debug("Failed flushing refresh stderr filter", "stage", stage.Id, "error", flushErr)
	}
	return refreshErr
}

// Output retrieves the OpenTofu output for the specified stage directory.
// It returns a map of output variable names to their values in JSON format.
//
// It runs on a quiet (stdout-discarded) instance: terraform-exec tees the
// captured `tofu output -json` to stdout, which would leak sensitive stage
// outputs (e.g. credentials produced by an earlier stage and consumed as input
// by a later one) to the terminal/logs. Callers receive the structured map and
// are responsible for any intentional, redacted display.
func (c *TofuClient) Output(ctx context.Context, stage schema.StageConfig) (map[string][]byte, error) {
	log.Debug("tofu output", "stage", stage)
	tf, err := c.getTfQuiet(stage)
	if err != nil {
		return nil, err
	}

	output, err := tf.Output(ctx)
	if err != nil {
		return nil, err
	}

	res := make(map[string][]byte)

	for k, v := range output {
		vp, _ := json.MarshalIndent(v.Value, "", " ")
		res[k] = vp
	}

	return res, nil
}

// setStageEnv sets the environment variables for the OpenTofu process based on the stage configuration.
func (c *TofuClient) setStageEnv(tf *tfexec.Terraform, stage schema.StageConfig) {
	env := util.OsEnvMap()

	if stage.Providers.Kubernetes {
		env = util.MergeMaps(env, map[string]string{
			"KUBE_CONFIG_PATH": c.cfg.Config.KubeconfigPath(),
		})
	}

	// Enable provider/module plugin caching to avoid re-downloading for every stage
	pluginCacheDir := c.pluginCacheDir()
	if pluginCacheDir != "" {
		env = util.MergeMaps(env, map[string]string{
			"TF_PLUGIN_CACHE_DIR": pluginCacheDir,
		})
	}

	if err := tf.SetEnv(env); err != nil {
		log.Warn("Failed to set tofu environment", "stage", stage, "err", err)
	}
}

// pluginCacheDir returns the path to the provider plugin cache directory,
// creating it if necessary. Returns empty string on failure.
func (c *TofuClient) pluginCacheDir() string {
	dir := filepath.Join(c.cfg.Config.Tmp, "plugin-cache")
	if err := os.MkdirAll(dir, 0750); err != nil {
		log.Warn("Failed to create plugin cache directory", "path", dir, "err", err)
		return ""
	}
	return dir
}

// ForceUnlock attempts to force-unlock a state lock for the specified stage.
// It extracts the lock ID from the error message and calls `tofu force-unlock`.
func (c *TofuClient) ForceUnlock(ctx context.Context, stage schema.StageConfig, lockID string) error {
	log.Warn("Attempting force-unlock of state", "stage", stage.Id, "lockID", lockID)
	tf, err := c.getTf(stage.Path)
	if err != nil {
		return err
	}
	c.setStageEnv(tf, stage)
	return tf.ForceUnlock(ctx, lockID)
}

// Import brings an existing infrastructure object under OpenTofu management by
// associating the resource at the given configuration address with its real-world
// ID. It runs `tofu import <address> <id>` for the specified stage, applying the
// same stage input variables used by plan/apply so that any provider/config
// interpolation resolves identically.
func (c *TofuClient) Import(ctx context.Context, stage schema.StageConfig, address string, id string) error {
	log.Info("tofu import", "stage", stage.Id, "address", address, "id", id)
	tf, err := c.getTf(stage.Path)
	if err != nil {
		return err
	}

	stageVarFile, cleanup, err := c.stageVarFile(ctx, stage)
	if err != nil {
		return err
	}
	defer cleanup()

	var opts []tfexec.ImportOption
	if !stage.OverrideVars {
		opts = append(opts, tfexec.VarFile(c.cfg.Config.TfVarFilePath()))
	}
	if stageVarFile != "" {
		opts = append(opts, tfexec.VarFile(stageVarFile))
	}

	c.setStageEnv(tf, stage)
	return tf.Import(ctx, address, id, opts...)
}

// clusterResidentTypePrefixes enumerates the OpenTofu resource type prefixes
// whose objects live INSIDE the Kubernetes cluster (and therefore vanish when
// the cluster itself is destroyed). They are safe to drop from state once the
// cluster is confirmed absent — unlike AWS-provider resources in the same stage
// (IAM roles, KMS keys, secrets) which must still be destroyed normally.
var clusterResidentTypePrefixes = []string{
	"helm_release",
	"kubernetes_",
	"kubectl_",
}

// isClusterResidentType reports whether a resource type refers to an in-cluster
// object managed via the Kubernetes/Helm providers.
func isClusterResidentType(resourceType string) bool {
	for _, p := range clusterResidentTypePrefixes {
		if strings.HasPrefix(resourceType, p) {
			return true
		}
	}
	return false
}

// StateRemoveOrphanedClusterResources removes only the in-cluster (Helm/Kubernetes)
// MANAGED resources from a stage's state, leaving AWS-provider resources intact.
//
// This is a safe, surgical state cleanup for mixed stages (e.g.
// prereqs, core) that hold both cloud and in-cluster resources. When the EKS
// cluster has already been destroyed, its in-cluster objects are gone but remain
// recorded in state; OpenTofu can neither refresh nor destroy them because the
// Kubernetes/Helm providers can no longer reach an API server. Dropping just
// those orphaned records lets the remaining cloud resources destroy normally and
// unblocks state backend teardown — without ever orphaning real AWS resources.
//
// It returns the number of resources removed. Data sources are never touched.
func (c *TofuClient) StateRemoveOrphanedClusterResources(ctx context.Context, stage schema.StageConfig) (int, error) {
	log.Info("Removing orphaned in-cluster resources from state", "stage", stage.Id)
	state, err := c.showState(ctx, stage)
	if err != nil {
		return 0, err
	}

	var addresses []string
	var walk func(mod *tfjson.StateModule)
	walk = func(mod *tfjson.StateModule) {
		if mod == nil {
			return
		}
		for _, res := range mod.Resources {
			if res.Mode == tfjson.ManagedResourceMode && isClusterResidentType(res.Type) {
				addresses = append(addresses, res.Address)
			}
		}
		for _, child := range mod.ChildModules {
			walk(child)
		}
	}
	if state != nil && state.Values != nil {
		walk(state.Values.RootModule)
	}

	if len(addresses) == 0 {
		log.Info("No orphaned in-cluster resources in state", "stage", stage.Id)
		return 0, nil
	}

	tf, err := c.getTf(stage.Path)
	if err != nil {
		return 0, err
	}
	c.setStageEnv(tf, stage)

	removed := 0
	for _, addr := range addresses {
		if err := tf.StateRm(ctx, addr); err != nil {
			log.Warn("Failed to remove orphaned resource from state (may already be gone)", "stage", stage.Id, "address", addr, "error", err)
			continue
		}
		log.Warn("Removed orphaned in-cluster resource from state", "stage", stage.Id, "address", addr)
		removed++
	}
	return removed, nil
}

// StateResourceView is a redacted, presentation-friendly snapshot of a single
// resource instance recorded in the OpenTofu state. Sensitive attribute values
// are replaced with a redaction marker so the view can be safely printed to a
// terminal or log without spilling secrets (tokens, passwords, helm values,
// etc.). It is returned by StateShow.
type StateResourceView struct {
	Address string                 // full state address, e.g. helm_release.quartz
	Mode    string                 // "managed" or "data"
	Type    string                 // resource type, e.g. helm_release
	Name    string                 // resource name, e.g. quartz
	Values  map[string]interface{} // attribute values with sensitive entries redacted
}

// StateList returns the addresses of every resource instance recorded in the
// state for the given stage, walking the root module and all child modules.
// It is the structured equivalent of `tofu state list` (which terraform-exec
// does not expose directly) and is derived from `tofu show -json`.
//
// An optional set of case-insensitive substring filters may be supplied; when
// non-empty, only addresses matching at least one filter are returned.
func (c *TofuClient) StateList(ctx context.Context, stage schema.StageConfig, filters ...string) ([]string, error) {
	log.Debug("tofu state list", "stage", stage.Id)
	state, err := c.showState(ctx, stage)
	if err != nil {
		return nil, err
	}

	addresses := collectStateAddresses(state)
	if len(filters) == 0 {
		return addresses, nil
	}

	var out []string
	for _, addr := range addresses {
		for _, f := range filters {
			if f == "" || strings.Contains(strings.ToLower(addr), strings.ToLower(f)) {
				out = append(out, addr)
				break
			}
		}
	}
	return out, nil
}

// StateShow returns redacted views of resource instances in the stage state.
// When addresses are supplied, only resources whose address exactly matches one
// of them are returned; otherwise every resource is returned. Sensitive values
// are masked. It is the structured, secret-safe equivalent of
// `tofu state show <address>`.
func (c *TofuClient) StateShow(ctx context.Context, stage schema.StageConfig, addresses ...string) ([]StateResourceView, error) {
	log.Debug("tofu state show", "stage", stage.Id, "addresses", addresses)
	state, err := c.showState(ctx, stage)
	if err != nil {
		return nil, err
	}

	want := make(map[string]bool, len(addresses))
	for _, a := range addresses {
		want[a] = true
	}

	var views []StateResourceView
	var walk func(mod *tfjson.StateModule)
	walk = func(mod *tfjson.StateModule) {
		if mod == nil {
			return
		}
		for _, res := range mod.Resources {
			if len(want) > 0 && !want[res.Address] {
				continue
			}
			views = append(views, StateResourceView{
				Address: res.Address,
				Mode:    string(res.Mode),
				Type:    res.Type,
				Name:    res.Name,
				Values:  redactStateValues(res.AttributeValues, res.SensitiveValues),
			})
		}
		for _, child := range mod.ChildModules {
			walk(child)
		}
	}
	if state != nil && state.Values != nil {
		walk(state.Values.RootModule)
	}
	return views, nil
}

// StateRemove removes the named resource instances from the stage state without
// destroying the underlying infrastructure. It is the equivalent of
// `tofu state rm <address>...` and is primarily used to drop orphaned resources
// (e.g. a helm_release pointing at an already-deleted cluster) so that a
// subsequent destroy/clean can proceed. Each address is removed independently;
// the first failure is returned after attempting the remainder so a partial
// batch still makes progress.
func (c *TofuClient) StateRemove(ctx context.Context, stage schema.StageConfig, addresses ...string) error {
	log.Info("tofu state rm", "stage", stage.Id, "addresses", addresses)
	if len(addresses) == 0 {
		return fmt.Errorf("no resource addresses provided")
	}

	tf, err := c.getTf(stage.Path)
	if err != nil {
		return err
	}
	c.setStageEnv(tf, stage)

	var firstErr error
	for _, addr := range addresses {
		if err := tf.StateRm(ctx, addr); err != nil {
			log.Warn("Failed to remove resource from state", "stage", stage.Id, "address", addr, "error", err)
			if firstErr == nil {
				firstErr = fmt.Errorf("failed to remove %s: %w", addr, err)
			}
			continue
		}
		log.Info("Removed resource from state", "stage", stage.Id, "address", addr)
	}
	return firstErr
}

// showState runs `tofu show -json` for the stage and returns the parsed state.
// It centralizes the getTf + env-prep + Show plumbing shared by the read-only
// state inspection helpers (StateList, StateShow).
func (c *TofuClient) showState(ctx context.Context, stage schema.StageConfig) (*tfjson.State, error) {
	tf, err := c.getTfQuiet(stage)
	if err != nil {
		return nil, err
	}
	state, err := tf.Show(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read state for stage %s: %w", stage.Id, err)
	}
	return state, nil
}

// getTfQuiet returns a stage-scoped OpenTofu instance whose stdout is discarded.
// It MUST be used for any command that captures JSON from stdout — most
// importantly `tofu show -json` (via Show()). terraform-exec tees the captured
// JSON to the instance's configured stdout (os.Stdout for the normal cached
// client), which would otherwise spill the ENTIRE state — including plaintext
// secrets such as tokens, passwords, and helm values — to the terminal/logs.
// Discarding stdout keeps the structured result (parsed from terraform-exec's
// internal buffer) intact while preventing the leak. stderr is preserved so
// genuine errors are still surfaced.
func (c *TofuClient) getTfQuiet(stage schema.StageConfig) (*tfexec.Terraform, error) {
	tf, err := c.newTfOpts(&TfOpts{dir: stage.Path, stdout: io.Discard, stderr: os.Stderr})
	if err != nil {
		return nil, err
	}
	c.setStageEnv(tf, stage)
	return tf, nil
}

// collectStateAddresses walks the root and child modules of a parsed state and
// returns every resource instance address in document order.
func collectStateAddresses(state *tfjson.State) []string {
	var addresses []string
	if state == nil || state.Values == nil {
		return addresses
	}
	var walk func(mod *tfjson.StateModule)
	walk = func(mod *tfjson.StateModule) {
		if mod == nil {
			return
		}
		for _, res := range mod.Resources {
			addresses = append(addresses, res.Address)
		}
		for _, child := range mod.ChildModules {
			walk(child)
		}
	}
	walk(state.Values.RootModule)
	return addresses
}

// redactStateValues returns a copy of a resource's attribute values with every
// value flagged sensitive replaced by "[REDACTED]". The sensitivity map mirrors
// the shape of the values tree (per the tofu show -json schema): a node is the
// boolean `true` when that attribute is sensitive, or a nested object/array
// describing sensitivity of nested attributes/elements. This guarantees secrets
// embedded anywhere in the tree (e.g. helm_release.values) are never printed.
func redactStateValues(values map[string]interface{}, sensitive json.RawMessage) map[string]interface{} {
	if values == nil {
		return nil
	}
	var sensTree interface{}
	if len(sensitive) > 0 {
		// Best-effort: if the sensitivity tree can't be parsed, fall back to
		// returning values unchanged rather than failing the whole show.
		_ = json.Unmarshal(sensitive, &sensTree)
	}
	redacted, _ := redactValue(values, sensTree).(map[string]interface{})
	return redacted
}

// redactValue recursively walks a value tree alongside its sensitivity tree and
// replaces sensitive leaves with a redaction marker.
func redactValue(value interface{}, sensitive interface{}) interface{} {
	// A sensitivity node of literal `true` redacts the entire subtree.
	if b, ok := sensitive.(bool); ok && b {
		return "[REDACTED]"
	}

	switch v := value.(type) {
	case map[string]interface{}:
		sensMap, _ := sensitive.(map[string]interface{})
		out := make(map[string]interface{}, len(v))
		for key, val := range v {
			var childSens interface{}
			if sensMap != nil {
				childSens = sensMap[key]
			}
			out[key] = redactValue(val, childSens)
		}
		return out
	case []interface{}:
		sensArr, _ := sensitive.([]interface{})
		out := make([]interface{}, len(v))
		for i, val := range v {
			var childSens interface{}
			if sensArr != nil && i < len(sensArr) {
				childSens = sensArr[i]
			}
			out[i] = redactValue(val, childSens)
		}
		return out
	default:
		return value
	}
}

// lockInfoIDPattern matches the "ID:" field of the OpenTofu "Lock Info:" block.
// It is anchored to the start of a line (after optional leading whitespace) so it
// does NOT match unrelated fields that merely end in "ID:" — most importantly the
// AWS DynamoDB "RequestID:" that appears earlier in a ConditionalCheckFailed error.
// Matching that request ID by accident would cause force-unlock to use the wrong
// ID and fail with "does not match existing lock", silently defeating recovery.
var lockInfoIDPattern = regexp.MustCompile(`(?m)^[ \t]*ID:[ \t]+(\S+)`)

// ExtractLockID extracts the OpenTofu state lock ID from a lock error message.
// Returns the lock ID and true if found, or empty string and false if not.
//
// The canonical lock error embeds the ID in a "Lock Info:" block:
//
//	Error: Error acquiring the state lock
//	...
//	Lock Info:
//	  ID:        54802a0b-4db5-819a-4f02-2827bdcf02ba
//	  Path:      ...
//
// We must extract the value from that "ID:" line specifically. A naive
// substring search for "ID:" matches "RequestID:" in the AWS error preamble
// first and returns the request ID instead of the lock ID.
func ExtractLockID(errMsg string) (string, bool) {
	m := lockInfoIDPattern.FindStringSubmatch(errMsg)
	if m == nil {
		return "", false
	}
	// Strip trailing punctuation (e.g. a trailing comma) that may follow the ID.
	lockID := strings.Trim(m[1], ",.;\"")
	if lockID == "" {
		return "", false
	}
	return lockID, true
}

// stageVarValues generates the input variables for the specified stage based on its configuration.
// It supports literal values, environment variables, configuration values, secrets, and outputs from other stages.
func (c *TofuClient) stageVarValues(ctx context.Context, stage schema.StageConfig) map[string]string {
	vars := make(map[string]string)

	outputs := make(map[string]map[string][]byte)

	log.Debug("Adding stage vars", "stage", stage)

	for k, v := range stage.Vars {
		if v.Value != "" {
			log.Debug("OpenTofu literal input var", "key", k, "val", redactSensitiveVar(k, v.Value))
			vars[k] = v.Value
		} else if v.Env != "" {
			val, found := os.LookupEnv(v.Env)
			if !found {
				log.Info("Stage env input not found", "stage", stage, "env", v.Env)
			}

			log.Debug("OpenTofu env input var", "key", v.Env, "val", redactSensitiveVar(k, val))
			vars[k] = val
		} else if v.Config != "" {
			val := c.cfg.ConfigString(v.Config)
			if val == "" {
				log.Info("Stage config input not found", "stage", stage, "config", v.Config)
				continue
			}

			log.Debug("OpenTofu config var", "key", v.Config, "val", redactSensitiveVar(k, val))
			vars[k] = val
		} else if v.Secret != "" {
			val := c.cfg.SecretString(v.Secret)
			if val == "" {
				log.Info("Stage secret input not found", "stage", stage, "secret", v.Secret)
				continue
			}

			log.Debug("OpenTofu secret input var", "key", v.Secret, "val", "[REDACTED]")
			vars[k] = val
		} else if v.Stage.Name != "" {
			_, ok := outputs[v.Stage.Name]
			if !ok {
				s := c.cfg.Config.Stages[v.Stage.Name]
				o, err := c.Output(ctx, s)
				if err != nil {
					log.Warn("Failed to get output", "stage", s.Id, "err", err)
					continue
				}

				outputs[v.Stage.Name] = o
			}

			val, err := parseStageOutputValue(outputs[v.Stage.Name], v.Stage.Output)
			if err != nil {
				log.Warn("Failed to parse stage output", "stage", v.Stage.Name, "output", v.Stage.Output)
				continue
			}

			log.Debug("OpenTofu stage input var", "stage", v.Stage.Name, "output", v.Stage.Output, "key", k, "val", redactSensitiveVar(k, val))
			vars[k] = val
		}
	}

	return vars
}

// stageVarFile writes stage variables to a temporary tfvars JSON file and
// returns its path plus a cleanup function. Passing only -var-file=<path> keeps
// secret values out of OpenTofu/terraform-exec DEBUG logs, which otherwise log
// full CLI arguments for -var name=value.
func (c *TofuClient) stageVarFile(ctx context.Context, stage schema.StageConfig) (string, func(), error) {
	values := c.stageVarValues(ctx, stage)
	cleanup := func() {}
	if len(values) == 0 {
		return "", cleanup, nil
	}

	dir := c.cfg.Config.Tmp
	if dir == "" {
		dir = os.TempDir()
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", cleanup, fmt.Errorf("failed to create tofu variable temp directory: %w", err)
	}

	f, err := os.CreateTemp(dir, "quartz-"+safeStageVarFilePrefix(stage.Id)+"-*.tfvars.json")
	if err != nil {
		return "", cleanup, fmt.Errorf("failed to create temporary tofu variable file: %w", err)
	}
	path := f.Name()
	cleanup = func() {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			log.Debug("Failed to remove temporary tofu variable file", "path", path, "err", err)
		}
	}

	if err := os.Chmod(path, 0o600); err != nil {
		_ = f.Close()
		cleanup()
		return "", func() {}, fmt.Errorf("failed to secure temporary tofu variable file: %w", err)
	}

	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(values); err != nil {
		_ = f.Close()
		cleanup()
		return "", func() {}, fmt.Errorf("failed to write temporary tofu variable file: %w", err)
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("failed to close temporary tofu variable file: %w", err)
	}

	log.Debug("OpenTofu stage vars written to temporary var-file", "stage", stage.Id, "path", path, "keys", len(values))
	return path, cleanup, nil
}

func safeStageVarFilePrefix(id string) string {
	safe := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, id)
	safe = strings.Trim(safe, "-")
	if safe == "" {
		return "stage"
	}
	return safe
}

// parseStageOutputValue parses a specific output value from the OpenTofu state of another stage.
// It supports nested keys using dot notation.
func parseStageOutputValue(o map[string][]byte, key string) (string, error) {
	before, after, found := strings.Cut(key, ".")

	v, f := o[before]
	if !f {
		return "", fmt.Errorf("key not found")
	}

	val := string(v)

	// if multiple components to the key, try to extract sub key(s)
	if found {
		r := gjson.Get(val, after)
		val = r.String()
	}

	return strings.Trim(val, "\""), nil
}

// targetsToDestroy determines the specific resources to destroy for the specified stage.
// It applies inclusion and exclusion filters based on the stage configuration.
//
// The state inspection runs on a quiet (stdout-discarded) OpenTofu instance via
// showState so the full `tofu show -json` output — which embeds plaintext
// secrets — is never streamed to the terminal/logs during destroy/clean.
func targetsToDestroy(ctx context.Context, c *TofuClient, stage schema.StageConfig) ([]string, bool, error) {
	hasIncludes := len(stage.Destroy.Include) > 0
	hasExcludes := len(stage.Destroy.Exclude) > 0

	if !hasIncludes && !hasExcludes {
		// nothing specified, default to a normal tofu destroy
		return nil, true, nil
	}

	state, err := c.showState(ctx, stage)
	if err != nil {
		return nil, false, err
	}

	if state == nil ||
		state.Values == nil ||
		state.Values.RootModule == nil ||
		(len(state.Values.RootModule.Resources) == 0 && len(state.Values.RootModule.ChildModules) == 0) {
		log.Debug("Empty state response from module", "stage", stage.Id, "state", state)
		return nil, false, nil
	}

	var targets []string
	checkResource := func(res *tfjson.StateResource) {
		log.Debug("Checking tofu state resource for explicit inclusion/exclusion in destroy operation", "address", res.Address)
		compFunc := func(e string) bool {
			return util.EqualsOrRegexMatchString(e, res.Address, true)
		}

		if hasExcludes && slices.ContainsFunc(stage.Destroy.Exclude, compFunc) {
			// exclude has priority, if it's in this list, skip it
			log.Debug("Matched entry in exclusion list, removing resource from destroy set", "address", res.Address)
			return
		}

		if hasIncludes && !slices.ContainsFunc(stage.Destroy.Include, compFunc) {
			// include has anything at all and doesn't contain this resource, skip it
			log.Debug("Did not match anything in inclusion list, removing resource from destroy set", "address", res.Address)
			return
		}

		// if we got this far, we want it included in the destroy
		log.Debug("Adding resource to destroy set", "address", res.Address)
		targets = append(targets, res.Address)
	}

	var checkModule func(*tfjson.StateModule)
	checkModule = func(mod *tfjson.StateModule) {
		for _, res := range mod.Resources {
			checkResource(res)
		}

		for _, mod := range mod.ChildModules {
			log.Debug("Recursively checking child module", "address", mod.Address)
			checkModule(mod)
		}
	}

	log.Debug("Checking root module", "address", state.Values.RootModule.Address)
	checkModule(state.Values.RootModule)

	// filters were applied but the result set was empty, skip the destroy
	if len(targets) == 0 {
		return nil, false, nil
	}

	targets = util.DistinctSlice(targets)

	return targets, true, nil
}
