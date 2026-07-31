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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MetroStar/quartzctl/internal/config"
	"github.com/MetroStar/quartzctl/internal/config/schema"
	"github.com/MetroStar/quartzctl/internal/log"
	"github.com/knadh/koanf/v2"
	"github.com/stretchr/testify/assert"
)

const (
	test_version = "1.11.6"
)

func TestTofuInstance(t *testing.T) {
	tf, err := sharedTestTfClient(t)
	if err != nil {
		t.Errorf("unexpected error from tofu client constructor, %v", err)
	}

	defer tf.Cleanup(context.Background())

	if tf.version != test_version ||
		!strings.HasSuffix(tf.execPath, "/tofu") {
		t.Errorf("incorrect tofu client config, expected %s %s, found %s %s", test_version, tf.cfg.Config.Tmp, tf.version, tf.execPath)
	}
}

func TestTofuCtor(t *testing.T) {
	tf, err := newTestTfClient(t)
	if err != nil {
		t.Errorf("unexpected error from tofu client constructor, %v", err)
	}

	defer tf.Cleanup(context.Background())

	if tf.version != test_version ||
		!strings.HasSuffix(tf.execPath, "/tofu") {
		t.Errorf("incorrect tofu client config, expected %s %s, found %s %s", test_version, tf.cfg.Config.Tmp, tf.version, tf.execPath)
	}
}

func TestTofuCleanupEmpty(t *testing.T) {
	tf := &TofuClient{}
	tf.Cleanup(context.Background())
}

func TestTofuCtorError(t *testing.T) {
	tmp := t.TempDir()
	_, err := NewTofuClient(context.Background(), config.Settings{
		Config: schema.QuartzConfig{
			Tmp: tmp,
			Tofu: schema.TofuConfig{
				Version: "999.9.9",
			},
		},
	})
	if err == nil {
		t.Error("expected error from tofu client constructor")
	}
}

func TestTofuVersion(t *testing.T) {
	tf, err := newTestTfClient(t)
	if err != nil {
		t.Errorf("unexpected error from tofu client constructor, %v", err)
	}

	defer tf.Cleanup(context.Background())

	actual, err := tf.Version(context.Background())
	if err != nil {
		t.Errorf("unexpected error from tofu version, %v", err)
	} else if actual != test_version {
		t.Errorf("incorrect tofu version, expected %s, found %s", test_version, actual)
	}
}

func TestTofuInit(t *testing.T) {
	tf, err := newTestTfClient(t)
	if err != nil {
		t.Errorf("unexpected error from tofu client constructor, %v", err)
	}

	defer tf.Cleanup(context.Background())

	stage := newSimpleStageConfig()
	err = tf.Init(context.Background(), stage, TofuInitOpts{})
	if err != nil {
		t.Errorf("unexpected error from tofu init, %v", err)
	}
}

func TestTofuValidate(t *testing.T) {
	tf, err := newTestTfClient(t)
	if err != nil {
		t.Errorf("unexpected error from tofu client constructor, %v", err)
	}

	defer tf.Cleanup(context.Background())

	stage := newSimpleStageConfig()
	tf.Init(context.Background(), stage, TofuInitOpts{})
	actual, err := tf.Validate(context.Background(), stage)
	if err != nil {
		t.Errorf("unexpected error from tofu validate, %v", err)
	}

	if actual.ErrorCount != 0 ||
		actual.WarningCount != 0 {
		t.Errorf("incorrect response from tofu validate, expected (%d %d), found (%d %d)", 0, 0, actual.WarningCount, actual.ErrorCount)
	}
}

func TestTofuFormat(t *testing.T) {
	tf, err := newTestTfClient(t)
	if err != nil {
		t.Errorf("unexpected error from tofu client constructor, %v", err)
	}

	defer tf.Cleanup(context.Background())

	stage := newSimpleStageConfig()
	tf.Init(context.Background(), stage, TofuInitOpts{})
	err = tf.Format(context.Background(), stage)
	if err != nil {
		t.Errorf("unexpected error from tofu format, %v", err)
	}
}

func TestTofuPlan(t *testing.T) {
	tf, err := newTestTfClient(t)
	if err != nil {
		t.Errorf("unexpected error from tofu client constructor, %v", err)
	}

	defer tf.Cleanup(context.Background())

	// boolean response will vary depending on if the module has been applied previously
	// or not
	stage := newSimpleStageConfig()
	tf.Init(context.Background(), stage, TofuInitOpts{})
	_, err = tf.Plan(context.Background(), stage)
	if err != nil {
		t.Errorf("unexpected error from tofu plan, %v", err)
	}
}

func TestTofuApply(t *testing.T) {
	tf, err := newTestTfClient(t)
	if err != nil {
		t.Errorf("unexpected error from tofu client constructor, %v", err)
	}

	defer tf.Cleanup(context.Background())

	stage := newSimpleStageConfig()
	tf.Init(context.Background(), stage, TofuInitOpts{})
	err = tf.Apply(context.Background(), stage)
	if err != nil {
		t.Errorf("unexpected error from tofu apply, %v", err)
	}
}

func TestTofuApplyTargets(t *testing.T) {
	t.Setenv("TEST_TF_INPUT_1", "testvalue1")

	tf, err := newTestTfClient(t)
	if err != nil {
		t.Errorf("unexpected error from tofu client constructor, %v", err)
	}

	defer tf.Cleanup(context.Background())

	stage := schema.StageConfig{
		Path: "./testdata/destroy",
		Vars: map[string]schema.StageVarsConfig{
			"env_input": {Env: "TEST_TF_INPUT_1"},
		},
	}

	tf.Init(context.Background(), stage, TofuInitOpts{})

	// A targeted apply must succeed and create only the targeted resource's
	// subgraph, leaving the others untouched. This mirrors the Flux-owned
	// release convergence path, where only the values overlay Secret is applied.
	err = tf.Apply(context.Background(), stage, TofuApplyOpts{
		Targets: []string{"random_integer.include"},
	})
	if err != nil {
		t.Errorf("unexpected error from targeted tofu apply, %v", err)
	}
}

func TestTofuDestroy(t *testing.T) {
	t.Setenv("TEST_TF_INPUT_1", "testvalue1")
	tf, err := newTestTfClient(t)
	if err != nil {
		t.Errorf("unexpected error from tofu client constructor, %v", err)
	}

	defer tf.Cleanup(context.Background())

	stage := schema.StageConfig{
		Path: "./testdata/destroy",
		Vars: map[string]schema.StageVarsConfig{
			"env_input": {Env: "TEST_TF_INPUT_1"},
		},
		Destroy: schema.StageDestroyConfig{
			Skip: false,
			Include: []string{
				"random_integer.include",
				"module.mod.this",
			},
			Exclude: []string{"random_integer.exclude"},
		},
	}

	tf.Init(context.Background(), stage, TofuInitOpts{})
	tf.Apply(context.Background(), stage)
	err = tf.Destroy(context.Background(), stage)
	if err != nil {
		t.Errorf("unexpected error from tofu destroy, %v", err)
		return
	}

	stage.Destroy.Skip = true
	err = tf.Destroy(context.Background(), stage)
	if err != nil {
		t.Errorf("unexpected error from tofu destroy, %v", err)
		return
	}

	stage.Debug.Break = true
	err = tf.Destroy(context.Background(), stage)
	if err == nil {
		t.Errorf("expected break in tofu destroy")
	}
}

func TestTofuRefresh(t *testing.T) {
	tf, err := newTestTfClient(t)
	if err != nil {
		t.Errorf("unexpected error from tofu client constructor, %v", err)
	}

	defer tf.Cleanup(context.Background())

	stage := newSimpleStageConfig()
	tf.Init(context.Background(), stage, TofuInitOpts{})
	err = tf.Refresh(context.Background(), stage)
	if err != nil {
		t.Errorf("unexpected error from tofu refresh, %v", err)
	}
}

func TestTofuOutput(t *testing.T) {
	tf, err := newTestTfClient(t)
	if err != nil {
		t.Errorf("unexpected error from tofu client constructor, %v", err)
	}

	defer tf.Cleanup(context.Background())

	stage := newSimpleStageConfig()
	tf.Init(context.Background(), stage, TofuInitOpts{})
	tf.Apply(context.Background(), stage)
	actual, err := tf.Output(context.Background(), stage)
	if err != nil {
		t.Errorf("unexpected error from tofu validate, %v", err)
		return
	}

	var1 := strings.Trim(string(actual["var1"]), `"`)
	if var1 != "my-test-cluster" {
		t.Errorf("incorrect response from tofu output, expected %s, found %s", "my-test-cluster", var1)
	}
}

func TestTofuApplyVars(t *testing.T) {
	t.Setenv("TEST_TF_INPUT_1", "testvalue1")

	tf, err := newTestTfClient(t)
	if err != nil {
		t.Errorf("unexpected error from tofu client constructor, %v", err)
	}

	defer tf.Cleanup(context.Background())

	tf.cfg.Config.Stages = map[string]schema.StageConfig{
		"prereq": {
			Path: "./testdata/prereq",
		},
	}

	prereq := schema.StageConfig{Path: "./testdata/prereq"}
	tf.Init(context.Background(), prereq, TofuInitOpts{})
	tf.Apply(context.Background(), prereq)

	stage := schema.StageConfig{
		Path: "./testdata/depends_on",
		Providers: schema.StageProvidersConfig{
			Kubernetes: true,
		},
		OverrideVars: true,
		Vars: map[string]schema.StageVarsConfig{
			"value_input":  {Value: "literal"},
			"env_input":    {Env: "TEST_TF_INPUT_1"},
			"config_input": {Config: "dns.domain"},
			"secret_input": {Secret: "foo.bar"},
			"stage_input": {
				Stage: schema.StageVarsStageConfig{
					Name:   "prereq",
					Output: "val.first",
				},
			},
			"config_not_found": {Config: "this.does.not.exist"},
			"secret_not_found": {Secret: "this.does.not.exist"},
			"stage_not_founc": {
				Stage: schema.StageVarsStageConfig{
					Name:   "doesnt",
					Output: "exist",
				},
			},
		},
	}
	tf.Init(context.Background(), stage, TofuInitOpts{BackendConfig: []string{"foo=bar"}})
	err = tf.Apply(context.Background(), stage)
	if err != nil {
		t.Errorf("unexpected error from tofu apply, %v", err)
	}
}

func TestNewTofuClient(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Settings{
		Config: schema.QuartzConfig{
			Tofu: schema.TofuConfig{Version: test_version},
			Tmp:  tmpDir,
		},
	}

	client, err := NewTofuClient(context.Background(), cfg)
	defer client.Cleanup(context.Background())
	assert.NoError(t, err, "NewTofuClient should not return an error")
	assert.NotNil(t, client, "TofuClient instance should not be nil")
	assert.Equal(t, test_version, client.version, "OpenTofu version should match")
}

func TestTofuClient_Cleanup(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Settings{
		Config: schema.QuartzConfig{
			Tofu: schema.TofuConfig{Version: test_version},
			Tmp:  tmpDir,
		},
	}

	client, err := NewTofuClient(context.Background(), cfg)
	assert.NoError(t, err, "NewTofuClient should not return an error")

	err = client.Cleanup(context.Background())
	assert.NoError(t, err, "Cleanup should not return an error")
}

func TestTofuClient_getTf(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Settings{
		Config: schema.QuartzConfig{
			Tofu: schema.TofuConfig{Version: test_version},
			Tmp:  tmpDir,
		},
	}

	client, err := NewTofuClient(context.Background(), cfg)
	defer client.Cleanup(context.Background())
	assert.NoError(t, err, "NewTofuClient should not return an error")

	tf, err := client.getTf(tmpDir)
	assert.NoError(t, err, "getTf should not return an error")
	assert.NotNil(t, tf, "Tofu instance should not be nil")
}

func TestTofuClient_newTfOpts(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Settings{
		Config: schema.QuartzConfig{
			Tofu: schema.TofuConfig{Version: test_version},
			Tmp:  tmpDir,
		},
	}

	client, err := NewTofuClient(context.Background(), cfg)
	defer client.Cleanup(context.Background())
	assert.NoError(t, err, "NewTofuClient should not return an error")

	opts := &TfOpts{
		dir:    tmpDir,
		stdout: os.Stdout,
		stderr: os.Stderr,
	}

	tf, err := client.newTfOpts(opts)
	assert.NoError(t, err, "newTfOpts should not return an error")
	assert.NotNil(t, tf, "Tofu instance should not be nil")
}

func TestInstall(t *testing.T) {
	tmpDir := t.TempDir()
	version := "1.11.6"

	execPath, err := install(context.Background(), version, tmpDir)
	assert.NoError(t, err, "install should not return an error")
	assert.NotEmpty(t, execPath, "execPath should not be empty")

	// Verify the installed OpenTofu binary exists
	_, err = os.Stat(filepath.Join(tmpDir, "tofu"))
	assert.NoError(t, err, "OpenTofu binary should exist in the specified directory")
}

func TestInitLog(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := schema.QuartzConfig{
		Name: "test",
		Log: log.LogOptionsConfig{
			Tofu: log.TofuLogConfig{
				Enabled: true,
				Path:    filepath.Join(tmpDir, "tofu.log"),
				Level:   "DEBUG",
			},
		},
	}

	tfc, err := newTestTfClient(t)
	if err != nil {
		t.Errorf("unexpected error from tofu client constructor, %v", err)
		return
	}

	defer tfc.Cleanup(context.Background())

	tf, err := tfc.getTf(t.TempDir())
	if err != nil {
		t.Errorf("unexpected error from tfexec client constructor, %v", err)
		return
	}

	initLog(tf, cfg)

	// do something to trigger logging
	tf.Version(context.Background(), true)

	// Verify the log file path was created
	_, err = os.Stat(filepath.Join(tmpDir, "tofu.log"))
	assert.NoError(t, err, "OpenTofu log file should exist")
}

// newSimpleStageConfig creates a simple stage configuration for testing purposes.
func newSimpleStageConfig() schema.StageConfig {
	return schema.StageConfig{
		Path: "./testdata/simple",
		Vars: map[string]schema.StageVarsConfig{
			"value_input": {Value: "literal"},
		},
	}
}

func setupTestTfClient(t *testing.T) (config.Settings, error) {
	tmp := t.TempDir()

	kc := koanf.New(".")
	kc.Set("name", "my-test-cluster")
	kc.Set("dns.domain", "my-test-cluster.example.com")
	kc.Set("tmp", tmp)
	kc.Set("tofu.version", test_version)
	kc.Set("log.tofu.enabled", true)
	kc.Set("log.tofu.path", filepath.Join(tmp, "log", "tf.test.log"))

	ks := koanf.New(".")
	ks.Set("foo.bar", "supersecretvalue")

	lcr, err := config.NewSettings(kc, ks)
	if err != nil {
		return lcr, err
	}

	return lcr, lcr.WriteJsonConfig(filepath.Join(tmp, "quartz.tfvars.json"), "settings", false)
}

func sharedTestTfClient(t *testing.T) (*TofuClient, error) {
	c, err := setupTestTfClient(t)
	if err != nil {
		return &TofuClient{}, err
	}

	// will panic on error
	tf := Instance(context.Background(), c)
	return tf, nil
}

func newTestTfClient(t *testing.T) (TofuClient, error) {
	c, err := setupTestTfClient(t)
	if err != nil {
		return TofuClient{}, err
	}

	return NewTofuClient(context.Background(), c)
}

func TestStageVarFileWritesSecretValuesOutsideCLIArgs(t *testing.T) {
	tf, err := newTestTfClient(t)
	if err != nil {
		t.Fatalf("unexpected tofu client constructor error, %v", err)
	}
	defer tf.Cleanup(context.Background())

	stage := schema.StageConfig{
		Id: "secret-stage",
		Vars: map[string]schema.StageVarsConfig{
			"github_token": {Secret: "foo.bar"},
			"domain":       {Config: "dns.domain"},
		},
	}

	path, cleanup, err := tf.stageVarFile(context.Background(), stage)
	if err != nil {
		t.Fatalf("stageVarFile returned error: %v", err)
	}
	defer cleanup()

	if path == "" {
		t.Fatal("stageVarFile returned empty path")
	}
	if !strings.Contains(filepath.Base(path), "secret-stage") {
		t.Fatalf("stage var file should include sanitized stage id, got %s", path)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stage var file missing: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("stage var file mode = %v, want 0600", got)
	}

	data, err := os.ReadFile(path) // #nosec G304 -- test-owned temp file
	if err != nil {
		t.Fatalf("failed reading stage var file: %v", err)
	}
	var values map[string]string
	if err := json.Unmarshal(data, &values); err != nil {
		t.Fatalf("stage var file is not JSON: %v", err)
	}
	if values["github_token"] != "supersecretvalue" {
		t.Fatalf("secret value was not written to temporary var-file")
	}
	if values["domain"] != "my-test-cluster.example.com" {
		t.Fatalf("config value was not written to temporary var-file")
	}
}
