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
	"strings"
	"time"

	"github.com/MetroStar/quartzctl/internal/config"
	"github.com/MetroStar/quartzctl/internal/log"
	"github.com/MetroStar/quartzctl/internal/provider"
	"github.com/urfave/cli/v3"
)

// CommandParams holds the configuration and runtime parameters for CLI commands.
//
// Fields:
//   - configFile: Path to the configuration file.
//   - secretsFile: Path to the secrets file.
//   - startTime: The time when the command execution started.
//   - settings: Lazy-loaded settings from the configuration file.
//   - provider: Lazy-loaded provider factory for managing resources.
type CommandParams struct {
	configFile  string
	secretsFile string
	startTime   time.Time

	settings *config.Settings
	provider *provider.ProviderFactory
}

// NewCommandParams creates a new instance of CommandParams.
//
// Parameters:
//   - startTime: The time when the command execution started.
//
// Returns:
//   - *CommandParams: A new instance of CommandParams.
func NewCommandParams(startTime time.Time) *CommandParams {
	return &CommandParams{
		startTime: startTime,
	}
}

// SetConfig sets the configuration file path for the command parameters.
//
// Parameters:
//   - configFile: The path to the configuration file.
func (p *CommandParams) SetConfig(configFile string) {
	p.configFile = configFile
}

// SetSecrets sets the secrets file path for the command parameters.
//
// Parameters:
//   - secretsFile: The path to the secrets file.
func (p *CommandParams) SetSecrets(secretsFile string) {
	p.secretsFile = secretsFile
}

// Settings lazy loads the settings from the configuration file.
//
// Returns:
//   - *config.Settings: The loaded settings from the configuration file.
func (p *CommandParams) Settings() *config.Settings {
	if p.settings == nil {
		// Load configuration and secrets into a settings struct
		cfg, err := config.Load(context.Background(), p.configFile, p.secretsFile)
		if err != nil {
			log.Error("Failed to parse config", "err", err)
		}
		p.settings = &cfg
	}

	return p.settings
}

// Provider lazy loads the provider factory.
//
// Returns:
//   - *provider.ProviderFactory: The provider factory for managing resources.
func (p *CommandParams) Provider() *provider.ProviderFactory {
	if p.provider == nil {
		p.provider = provider.NewProviderFactory(p.Settings().Config, p.Settings().Secrets)
	}

	return p.provider
}

// ByCommandName compares two CLI commands by their names.
//
// Parameters:
//   - a: The first CLI command.
//   - b: The second CLI command.
//
// Returns:
//   - int: A negative number if a.Name < b.Name, zero if a.Name == b.Name, or a positive number if a.Name > b.Name.
func ByCommandName(a, b *cli.Command) int {
	return strings.Compare(a.Name, b.Name)
}
