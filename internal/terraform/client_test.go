package terraform

import (
	"context"
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
	test_version = "1.5.7"
)

func TestTerraformInstance(t *testing.T) {
	tf, err := sharedTestTfClient(t)
	if err != nil {
		t.Errorf("unexpected error from terraform client constructor, %v", err)
	}

	defer tf.Cleanup(context.Background())

	if tf.version != test_version ||
		!strings.HasSuffix(tf.execPath, "/terraform") {
		t.Errorf("incorrect terraform client config, expected %s %s, found %s %s", test_version, tf.cfg.Config.Tmp, tf.version, tf.execPath)
	}
}

func TestTerraformCtor(t *testing.T) {
	tf, err := newTestTfClient(t)
	if err != nil {
		t.Errorf("unexpected error from terraform client constructor, %v", err)
	}

	defer tf.Cleanup(context.Background())

	if tf.version != test_version ||
		!strings.HasSuffix(tf.execPath, "/terraform") {
		t.Errorf("incorrect terraform client config, expected %s %s, found %s %s", test_version, tf.cfg.Config.Tmp, tf.version, tf.execPath)
	}
}

func TestTerraformCleanupEmpty(t *testing.T) {
	tf := &TerraformClient{}
	tf.Cleanup(context.Background())
}

func TestTerraformCtorError(t *testing.T) {
	tmp := t.TempDir()
	_, err := NewTerraformClient(context.Background(), config.Settings{
		Config: schema.QuartzConfig{
			Tmp: tmp,
			Terraform: schema.TerraformConfig{
				Version: "999.9.9",
			},
		},
	})
	if err == nil {
		t.Error("expected error from terraform client constructor")
	}
}

func TestTerraformVersion(t *testing.T) {
	tf, err := newTestTfClient(t)
	if err != nil {
		t.Errorf("unexpected error from terraform client constructor, %v", err)
	}

	defer tf.Cleanup(context.Background())

	actual, err := tf.Version(context.Background())
	if err != nil {
		t.Errorf("unexpected error from terraform version, %v", err)
	} else if actual != test_version {
		t.Errorf("incorrect terraform version, expected %s, found %s", test_version, actual)
	}
}

func TestTerraformInit(t *testing.T) {
	tf, err := newTestTfClient(t)
	if err != nil {
		t.Errorf("unexpected error from terraform client constructor, %v", err)
	}

	defer tf.Cleanup(context.Background())

	stage := newSimpleStageConfig()
	err = tf.Init(context.Background(), stage, TerraformInitOpts{})
	if err != nil {
		t.Errorf("unexpected error from terraform init, %v", err)
	}
}

func TestTerraformValidate(t *testing.T) {
	tf, err := newTestTfClient(t)
	if err != nil {
		t.Errorf("unexpected error from terraform client constructor, %v", err)
	}

	defer tf.Cleanup(context.Background())

	stage := newSimpleStageConfig()
	tf.Init(context.Background(), stage, TerraformInitOpts{})
	actual, err := tf.Validate(context.Background(), stage)
	if err != nil {
		t.Errorf("unexpected error from terraform validate, %v", err)
	}

	if actual.ErrorCount != 0 ||
		actual.WarningCount != 0 {
		t.Errorf("incorrect response from terraform validate, expected (%d %d), found (%d %d)", 0, 0, actual.WarningCount, actual.ErrorCount)
	}
}

func TestTerraformFormat(t *testing.T) {
	tf, err := newTestTfClient(t)
	if err != nil {
		t.Errorf("unexpected error from terraform client constructor, %v", err)
	}

	defer tf.Cleanup(context.Background())

	stage := newSimpleStageConfig()
	tf.Init(context.Background(), stage, TerraformInitOpts{})
	err = tf.Format(context.Background(), stage)
	if err != nil {
		t.Errorf("unexpected error from terraform format, %v", err)
	}
}

func TestTerraformPlan(t *testing.T) {
	tf, err := newTestTfClient(t)
	if err != nil {
		t.Errorf("unexpected error from terraform client constructor, %v", err)
	}

	defer tf.Cleanup(context.Background())

	// boolean response will vary depending on if the module has been applied previously
	// or not
	stage := newSimpleStageConfig()
	tf.Init(context.Background(), stage, TerraformInitOpts{})
	_, err = tf.Plan(context.Background(), stage)
	if err != nil {
		t.Errorf("unexpected error from terraform plan, %v", err)
	}
}

func TestTerraformApply(t *testing.T) {
	tf, err := newTestTfClient(t)
	if err != nil {
		t.Errorf("unexpected error from terraform client constructor, %v", err)
	}

	defer tf.Cleanup(context.Background())

	stage := newSimpleStageConfig()
	tf.Init(context.Background(), stage, TerraformInitOpts{})
	err = tf.Apply(context.Background(), stage)
	if err != nil {
		t.Errorf("unexpected error from terraform apply, %v", err)
	}
}

func TestTerraformDestroy(t *testing.T) {
	t.Setenv("TEST_TF_INPUT_1", "testvalue1")
	tf, err := newTestTfClient(t)
	if err != nil {
		t.Errorf("unexpected error from terraform client constructor, %v", err)
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

	tf.Init(context.Background(), stage, TerraformInitOpts{})
	tf.Apply(context.Background(), stage)
	err = tf.Destroy(context.Background(), stage)
	if err != nil {
		t.Errorf("unexpected error from terraform destroy, %v", err)
		return
	}

	stage.Destroy.Skip = true
	err = tf.Destroy(context.Background(), stage)
	if err != nil {
		t.Errorf("unexpected error from terraform destroy, %v", err)
		return
	}

	stage.Debug.Break = true
	err = tf.Destroy(context.Background(), stage)
	if err == nil {
		t.Errorf("expected break in terraform destroy")
	}
}

func TestTerraformRefresh(t *testing.T) {
	tf, err := newTestTfClient(t)
	if err != nil {
		t.Errorf("unexpected error from terraform client constructor, %v", err)
	}

	defer tf.Cleanup(context.Background())

	stage := newSimpleStageConfig()
	tf.Init(context.Background(), stage, TerraformInitOpts{})
	err = tf.Refresh(context.Background(), stage)
	if err != nil {
		t.Errorf("unexpected error from terraform refresh, %v", err)
	}
}

func TestTerraformOutput(t *testing.T) {
	tf, err := newTestTfClient(t)
	if err != nil {
		t.Errorf("unexpected error from terraform client constructor, %v", err)
	}

	defer tf.Cleanup(context.Background())

	stage := newSimpleStageConfig()
	tf.Init(context.Background(), stage, TerraformInitOpts{})
	tf.Apply(context.Background(), stage)
	actual, err := tf.Output(context.Background(), stage)
	if err != nil {
		t.Errorf("unexpected error from terraform validate, %v", err)
		return
	}

	var1 := strings.Trim(string(actual["var1"]), `"`)
	if var1 != "my-test-cluster" {
		t.Errorf("incorrect response from terraform output, expected %s, found %s", "my-test-cluster", var1)
	}
}

func TestTerraformApplyVars(t *testing.T) {
	t.Setenv("TEST_TF_INPUT_1", "testvalue1")

	tf, err := newTestTfClient(t)
	if err != nil {
		t.Errorf("unexpected error from terraform client constructor, %v", err)
	}

	defer tf.Cleanup(context.Background())

	tf.cfg.Config.Stages = map[string]schema.StageConfig{
		"prereq": {
			Path: "./testdata/prereq",
		},
	}

	prereq := schema.StageConfig{Path: "./testdata/prereq"}
	tf.Init(context.Background(), prereq, TerraformInitOpts{})
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
	tf.Init(context.Background(), stage, TerraformInitOpts{BackendConfig: []string{"foo=bar"}})
	err = tf.Apply(context.Background(), stage)
	if err != nil {
		t.Errorf("unexpected error from terraform apply, %v", err)
	}
}

func TestNewTerraformClient(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Settings{
		Config: schema.QuartzConfig{
			Terraform: schema.TerraformConfig{Version: "1.0.0"},
			Tmp:       tmpDir,
		},
	}

	client, err := NewTerraformClient(context.Background(), cfg)
	defer client.Cleanup(context.Background())
	assert.NoError(t, err, "NewTerraformClient should not return an error")
	assert.NotNil(t, client, "TerraformClient instance should not be nil")
	assert.Equal(t, "1.0.0", client.version, "Terraform version should match")
}

func TestTerraformClient_Cleanup(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Settings{
		Config: schema.QuartzConfig{
			Terraform: schema.TerraformConfig{Version: "1.0.0"},
			Tmp:       tmpDir,
		},
	}

	client, err := NewTerraformClient(context.Background(), cfg)
	assert.NoError(t, err, "NewTerraformClient should not return an error")

	err = client.Cleanup(context.Background())
	assert.NoError(t, err, "Cleanup should not return an error")
}

func TestTerraformClient_getTf(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Settings{
		Config: schema.QuartzConfig{
			Terraform: schema.TerraformConfig{Version: "1.0.0"},
			Tmp:       tmpDir,
		},
	}

	client, err := NewTerraformClient(context.Background(), cfg)
	defer client.Cleanup(context.Background())
	assert.NoError(t, err, "NewTerraformClient should not return an error")

	tf, err := client.getTf(tmpDir)
	assert.NoError(t, err, "getTf should not return an error")
	assert.NotNil(t, tf, "Terraform instance should not be nil")
}

func TestTerraformClient_newTfOpts(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := config.Settings{
		Config: schema.QuartzConfig{
			Terraform: schema.TerraformConfig{Version: "1.0.0"},
			Tmp:       tmpDir,
		},
	}

	client, err := NewTerraformClient(context.Background(), cfg)
	defer client.Cleanup(context.Background())
	assert.NoError(t, err, "NewTerraformClient should not return an error")

	opts := &TfOpts{
		dir:    tmpDir,
		stdout: os.Stdout,
		stderr: os.Stderr,
	}

	tf, err := client.newTfOpts(opts)
	assert.NoError(t, err, "newTfOpts should not return an error")
	assert.NotNil(t, tf, "Terraform instance should not be nil")
}

func TestInstall(t *testing.T) {
	tmpDir := t.TempDir()
	version := "1.0.0"

	execPath, installer, err := install(context.Background(), version, tmpDir)
	assert.NoError(t, err, "install should not return an error")
	assert.NotEmpty(t, execPath, "execPath should not be empty")
	assert.NotNil(t, installer, "Installer instance should not be nil")

	// Verify the installed Terraform binary exists
	_, err = os.Stat(filepath.Join(tmpDir, "terraform"))
	assert.NoError(t, err, "Terraform binary should exist in the specified directory")
}

func TestInitLog(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := schema.QuartzConfig{
		Name: "test",
		Log: log.LogOptionsConfig{
			Terraform: log.TerraformLogConfig{
				Enabled: true,
				Path:    filepath.Join(tmpDir, "terraform.log"),
				Level:   "DEBUG",
			},
		},
	}

	tfc, err := newTestTfClient(t)
	if err != nil {
		t.Errorf("unexpected error from terraform client constructor, %v", err)
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
	_, err = os.Stat(filepath.Join(tmpDir, "terraform.log"))
	assert.NoError(t, err, "Terraform log file should exist")
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
	kc.Set("terraform.version", test_version)
	kc.Set("log.terraform.enabled", true)
	kc.Set("log.terraform.path", filepath.Join(tmp, "log", "tf.test.log"))

	ks := koanf.New(".")
	ks.Set("foo.bar", "supersecretvalue")

	lcr, err := config.NewSettings(kc, ks)
	if err != nil {
		return lcr, err
	}

	return lcr, lcr.WriteJsonConfig(filepath.Join(tmp, "quartz.tfvars.json"), "settings", false)
}

func sharedTestTfClient(t *testing.T) (*TerraformClient, error) {
	c, err := setupTestTfClient(t)
	if err != nil {
		return &TerraformClient{}, err
	}

	// will panic on error
	tf := Instance(context.Background(), c)
	return tf, nil
}

func newTestTfClient(t *testing.T) (TerraformClient, error) {
	c, err := setupTestTfClient(t)
	if err != nil {
		return TerraformClient{}, err
	}

	return NewTerraformClient(context.Background(), c)
}
