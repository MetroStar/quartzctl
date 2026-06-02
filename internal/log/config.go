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

package log

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
)

// LogConfig represents the configuration for logging, including log name and options.
type LogConfig struct {
	Name string           `koanf:"name"`
	Log  LogOptionsConfig `koanf:"log"`
}

// LogOptionsConfig contains configuration options for different logging outputs.
type LogOptionsConfig struct {
	Console ConsoleLogConfig `koanf:"console"`
	File    FileLogConfig    `koanf:"file"`
	Tofu    TofuLogConfig    `koanf:"tofu"`
}

// ConsoleLogConfig represents the configuration for console logging.
type ConsoleLogConfig struct {
	Level string `koanf:"level"`
}

// FileLogConfig represents the configuration for file-based logging.
type FileLogConfig struct {
	Enabled bool   `koanf:"enabled"`
	Path    string `koanf:"path"`
	Level   string `koanf:"level"`
}

// TofuLogConfig represents the configuration for OpenTofu-specific logging.
type TofuLogConfig struct {
	Enabled bool   `koanf:"enabled"`
	Path    string `koanf:"path"`
	Level   string `koanf:"level"`
}

// DefaultLogConfig provides the default logging configuration.
var DefaultLogConfig = LogConfig{
	Log: LogOptionsConfig{
		Console: ConsoleLogConfig{
			Level: "error",
		},
		File: FileLogConfig{
			Enabled: false,
			Path:    "log/$name.$date.log",
			Level:   "info",
		},
		Tofu: TofuLogConfig{
			Enabled: false,
			Path:    "log/$name.$date.tf.log",
			// `debug` rather than `trace`: TRACE emits the full provider RPC and
			// HTTP wire firehose, which has produced single-run logs of 750MB+.
			// DEBUG keeps the actionable plan/apply/provider detail at a fraction
			// of the size. Set to `trace` explicitly only when chasing a
			// provider-level bug.
			Level: "debug",
		},
	},
}

// DebugEnvOverride checks if the DEBUG environment variable is set to "true" or "1".
// If set, all loggers will be overridden to debug level regardless of the configured thresholds.
func DebugEnvOverride() bool {
	key := "DEBUG"
	if v := os.Getenv(key); slices.Contains([]string{"true", "1"}, strings.ToLower(v)) {
		return true
	}
	return false
}

// ConfigureDefault configures the default logger using the provided configuration file and writer.
// If an error occurs during configuration, it falls back to the default logger.
func ConfigureDefault(configFile string, w io.Writer) {
	cfg, err := NewLogConfig(configFile)
	if err != nil {
		// Log the error and fall back to the default configuration.
		Debug("Failed to load log configuration", "error", err)
		cfg = DefaultLogConfig
	}
	SetDefault(NewZapLogger(cfg, w))
}

// NewLogConfig loads the logging configuration from the specified file path.
// If the path is empty or an error occurs, it returns the default configuration.
func NewLogConfig(path string) (LogConfig, error) {
	k := koanf.New(".")
	k.Load(structs.Provider(DefaultLogConfig, "koanf"), nil)

	if path != "" {
		if err := k.Load(file.Provider(path), yaml.Parser()); err != nil {
			return DefaultLogConfig, err
		}
	}

	var w LogConfig
	if err := k.Unmarshal("", &w); err != nil {
		fmt.Printf("Error unmarshalling log config, %v\n", err)
		return DefaultLogConfig, err
	}

	return w, nil
}
