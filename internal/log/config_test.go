package log

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLogNewLogConfigDefault(t *testing.T) {
	tmp := t.TempDir()
	cfgContent := []byte(`
log: {}
`)
	cfgFile := filepath.Join(tmp, "test-config.yaml")
	fmt.Printf("Using %s\n", cfgFile)
	os.WriteFile(cfgFile, cfgContent, 0664)

	conf, err := NewLogConfig(cfgFile)
	if err != nil {
		t.Errorf("failed loading config, %v", err)
		return
	}

	if conf.Log.Console.Level != "error" ||
		conf.Log.File.Level != "info" {
		t.Errorf("incorrect log config, found %v", conf)
	}
}

func TestLogNewLogConfigFull(t *testing.T) {
	tmp := t.TempDir()
	cfgContent := []byte(`
log:
  console:
    level: debug
  file:
    enabled: true
    level: error
    path: ./testlog
`)
	cfgFile := filepath.Join(tmp, "test-config.yaml")
	fmt.Printf("Using %s\n", cfgFile)
	os.WriteFile(cfgFile, cfgContent, 0664)

	conf, err := NewLogConfig(cfgFile)
	if err != nil {
		t.Errorf("failed loading config, %v", err)
		return
	}

	if conf.Log.Console.Level != "debug" ||
		conf.Log.File.Enabled != true ||
		conf.Log.File.Level != "error" ||
		conf.Log.File.Path != "./testlog" {
		t.Errorf("incorrect log config, found %v", conf)
	}
}

func TestDefaultLogConfig_ConsoleLogConfig(t *testing.T) {
	assert.Equal(t, "error", DefaultLogConfig.Log.Console.Level, "Default console log level should be 'error'")
}

func TestDefaultLogConfig_FileLogConfig(t *testing.T) {
	assert.False(t, DefaultLogConfig.Log.File.Enabled, "Default file logging should be disabled")
	assert.Equal(t, "log/$name.$date.log", DefaultLogConfig.Log.File.Path, "Default file log path should match")
	assert.Equal(t, "info", DefaultLogConfig.Log.File.Level, "Default file log level should be 'info'")
}

func TestDefaultLogConfig_TerraformLogConfig(t *testing.T) {
	assert.False(t, DefaultLogConfig.Log.Terraform.Enabled, "Default Terraform logging should be disabled")
	assert.Equal(t, "log/$name.$date.tf.log", DefaultLogConfig.Log.Terraform.Path, "Default Terraform log path should match")
	assert.Equal(t, "trace", DefaultLogConfig.Log.Terraform.Level, "Default Terraform log level should be 'trace'")
}

func TestNewLogConfig_ReturnsDefaultOnEmptyPath(t *testing.T) {
	config, err := NewLogConfig("")
	assert.NoError(t, err, "NewLogConfig should not return an error for an empty path")
	assert.Equal(t, DefaultLogConfig, config, "NewLogConfig should return the default configuration for an empty path")
}

func TestNewLogConfig_LoadsFromFile(t *testing.T) {
	tmp := t.TempDir()
	cfgContent := []byte(`
name: test-log-config
log:
  console:
    level: debug
  file:
    enabled: true
    path: /var/log/test.log
    level: warn
  terraform:
    enabled: true
    path: /var/log/terraform.log
    level: info
`)
	cfgFile := filepath.Join(tmp, "log-config.yaml")
	err := os.WriteFile(cfgFile, cfgContent, 0664)
	assert.NoError(t, err, "Failed to write test log config file")

	config, err := NewLogConfig(cfgFile)
	assert.NoError(t, err, "NewLogConfig should not return an error for a valid file path")

	assert.Equal(t, "test-log-config", config.Name, "LogConfig name should match the file content")
	assert.Equal(t, "debug", config.Log.Console.Level, "Console log level should match the file content")
	assert.True(t, config.Log.File.Enabled, "File logging should be enabled as per the file content")
	assert.Equal(t, "/var/log/test.log", config.Log.File.Path, "File log path should match the file content")
	assert.Equal(t, "warn", config.Log.File.Level, "File log level should match the file content")
	assert.True(t, config.Log.Terraform.Enabled, "Terraform logging should be enabled as per the file content")
	assert.Equal(t, "/var/log/terraform.log", config.Log.Terraform.Path, "Terraform log path should match the file content")
	assert.Equal(t, "info", config.Log.Terraform.Level, "Terraform log level should match the file content")
}

func TestNewLogConfig_ReturnsDefaultOnInvalidFile(t *testing.T) {
	tmp := t.TempDir()
	cfgFile := filepath.Join(tmp, "invalid-log-config.yaml")

	// Write an invalid YAML file
	err := os.WriteFile(cfgFile, []byte("invalid_yaml: ["), 0664)
	assert.NoError(t, err, "Failed to write invalid test log config file")

	config, err := NewLogConfig(cfgFile)
	assert.Error(t, err, "NewLogConfig should return an error for an invalid file")
	assert.Equal(t, DefaultLogConfig, config, "NewLogConfig should return the default configuration for an invalid file")
}
