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

// Package config implements parsing and serialization
// for the quartz.yaml, environment and secrets required
// by the platform
package config

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"

	"github.com/MetroStar/quartzctl/internal/config/schema"
	"github.com/MetroStar/quartzctl/internal/log"
	"github.com/MetroStar/quartzctl/internal/provider"
	"github.com/MetroStar/quartzctl/internal/stages"
)

// Load reads the configuration and secrets files and parses them into a Settings instance.
func Load(ctx context.Context, configFile string, secretsFile string) (Settings, error) {
	k, err := LoadRawConfig(ctx, configFile)
	if err != nil {
		return Settings{}, err
	}

	s, err := LoadRawSecrets(ctx, secretsFile)
	if err != nil {
		return Settings{}, err
	}

	return NewSettings(k, s)
}

// LoadRawConfig reads the specified configuration file and processes it into a Koanf map.
// It applies defaults, environment variables, and additional settings.
func LoadRawConfig(ctx context.Context, configFile string) (*koanf.Koanf, error) {
	k := koanf.New(".")

	// set initial defaults
	setDefaults(k)

	// First pass of environment variables in case needed as inputs elsewhere
	loadDefaultEnvironment(k)

	// load quartz.yaml
	if err := k.Load(file.Provider(configFile), yaml.Parser()); err != nil {
		// could technically configure everything from other sources but unlikely?
		log.Warn("No config file found", "path", configFile, "err", err)
	}

	if err := checkCloudConfig(ctx, k); err != nil {
		return nil, err
	}

	// fill in any defaults that require input
	err := setDnsDefaults(k)
	if err != nil {
		return nil, err
	}

	setGitopsDefaults(k)
	setCoreDefaults(k)
	setAppDefaults(k)
	setAuthDefaults(k)

	// parse stages
	loadStages(k)

	// Second pass of environment variables to ensure precedence
	loadDefaultEnvironment(k)

	tmp, err := initTmpDir(k)
	if err != nil {
		log.Warn("Failed to create tmp directory", "dir", tmp, "err", err)
	}

	return k, nil
}

// LoadRawSecrets reads the secrets file and processes it into a Koanf map.
// It also loads credentials from environment variables.
func LoadRawSecrets(ctx context.Context, secretsFile string) (*koanf.Koanf, error) {
	s := koanf.New(".")

	loadCredentialsEnvironment(s)

	if err := s.Load(file.Provider(secretsFile), yaml.Parser()); err != nil {
		log.Debug("No secrets config file found", "path", secretsFile, "err", err)
	}

	return s, nil
}

// loadDefaultEnvironment loads configuration values from environment variables prefixed with "QUARTZ_".
func loadDefaultEnvironment(k *koanf.Koanf) {
	err := k.Load(env.Provider("QUARTZ_", ".", func(s string) string {
		return strings.ReplaceAll(strings.ToLower(
			strings.TrimPrefix(s, "QUARTZ_")), "_", ".")
	}), nil)

	if err != nil {
		log.Warn("Failed to load config from environment", "err", err)
	}
}

// setDnsDefaults sets default values for DNS configuration.
// It ensures that at least one of `dns.zone` or `dns.domain` is specified.
func setDnsDefaults(k *koanf.Koanf) error {
	zone := k.String("dns.zone")
	domain := k.String("dns.domain")

	if zone == "" && domain == "" {
		return fmt.Errorf("at least one of dns.zone or dns.domain must be specified")
	}

	// assume zone is the remainder of the domain
	if zone == "" {
		s := strings.SplitN(domain, ".", 2)
		return k.Set("dns.zone", s[1])
	}

	// otherwise prepend the cluster name to the zone for the full domain
	name := k.String("name")
	return k.Set("dns.domain", name+"."+zone)
}

// loadCredentialsEnvironment loads credentials from environment variables into the Koanf map.
func loadCredentialsEnvironment(k *koanf.Koanf) {
	e := map[string][]string{
		"ironbank.username":     {"IRONBANK_USERNAME", "REGISTRY_USERNAME"},
		"ironbank.password":     {"IRONBANK_PASSWORD", "REGISTRY_PASSWORD"},
		"ironbank.email":        {"IRONBANK_EMAIL", "REGISTRY_EMAIL"},
		"github.username":       {"GITHUB_USERNAME"},
		"github.token":          {"GITHUB_TOKEN"},
		"cloudflare.account_id": {"CLOUDFLARE_ACCOUNT_ID"},
		"cloudflare.api_token":  {"CLOUDFLARE_API_TOKEN", "CLOUDFLARE_TOKEN"},
		"cloudflare.email":      {"CLOUDFLARE_EMAIL"},
	}
	for key, envs := range e {
		if k.String(key) != "" {
			// config value already set, skip
			continue
		}

		for _, v := range envs {
			val := os.Getenv(v)
			if val == "" {
				// environment variable missing/empty, skip
				continue
			}

			// set the value and move on
			if err := k.Set(key, val); err != nil {
				log.Warn("Failed to set credentials from environment variable", "key", key, "env", v, "err", err)
				continue
			}

			// it worked, move to the next key
			break
		}
	}
}

// setDefaults sets default values for the Quartz configuration.
func setDefaults(k *koanf.Koanf) {
	providers := schema.NewProvidersConfig()
	pwd, _ := os.Getwd()

	err := k.Load(structs.Provider(schema.QuartzConfig{
		Project:      "quartz",
		Chart:        schema.ChartConfig{Path: filepath.Join(pwd, "base")},
		Providers:    providers,
		Terraform:    schema.NewTerraformConfig(),
		Auth:         schema.DefaultAuthConfig(),
		Gitops:       schema.DefaultGitopsConfig(providers.SourceControl),
		Github:       schema.NewGithubConfig(),
		Environments: schema.DefaultApplicationEnvironments(),
		Core:         schema.NewInfrastructureEnvironmentConfig("infra", "Quartz"),
		Mirror:       schema.NewMirrorConfig(),
		StagePaths:   []string{filepath.Join(pwd, "terraform", "stages")},
		Export:       schema.NewExportConfig(),
		State:        schema.NewStateConfig(),
		Log:          log.DefaultLogConfig.Log,
		Internal:     schema.NewInternalConfig(),
	}, "koanf"), nil)

	if err != nil {
		log.Warn("Failed to set default config", "err", err)
	}
}

// checkCloudConfig validates the cloud provider configuration.
func checkCloudConfig(ctx context.Context, k *koanf.Koanf) error {
	p := k.String("providers.cloud")

	pc, err := provider.NewCloudProviderClientWithOpts(ctx, provider.CloudProviderClientOpts{
		Provider: p,
	})
	if err != nil {
		return err
	}

	return pc.CheckConfig()
}

// setGitopsDefaults sets default values for GitOps configuration.
func setGitopsDefaults(k *koanf.Koanf) {
	name := k.String("name")
	p := k.String("providers.source_control")

	var sc schema.GithubConfig
	if err := k.Unmarshal(p, &sc); err != nil {
		log.Warn("Failed to unmarshal source control config", "provider", p, "err", err)
	}

	var gitops schema.GitopsConfig
	if err := k.Unmarshal("gitops", &gitops); err != nil {
		log.Warn("Failed to unmarshal gitops config", "err", err)
	}

	org := sc.Organization

	if gitops.Apps.Branch == "" {
		gitops.Apps.Branch = name
	}

	for _, r := range []*schema.RepositoryConfig{&gitops.Core, &gitops.Apps} {
		if r.Provider == "" {
			r.Provider = p
		}

		if r.Organization == "" {
			r.Organization = org
		}

		if r.Branch == "" {
			r.Branch = "main"
		}

		if r.RepoUrl == "" {
			r.RepoUrl = githubRepoUrl(r.Organization, r.Name)
		}
	}

	k2 := koanf.New(".")
	k2.Load(structs.Provider(gitops, "koanf"), nil)
	k.MergeAt(k2, "gitops")
}

// setCoreDefaults sets default values for core infrastructure applications.
// It disables Grafana-related applications if Grafana is not the selected monitoring provider.
func setCoreDefaults(k *koanf.Koanf) {
	apps := make(map[string]schema.InfrastructureApplicationConfig)
	k.Unmarshal("core.applications", &apps)

	grafanaApps := []string{"grafana", "tempo", "loki", "alertmanager"}
	grafanaEnabled := k.String("providers.monitoring") == "grafana"

	// disable configuration of any core apps related to the grafana/loki/tempo stack if we're not using it
	for key, app := range apps {
		fullKey := "core.applications." + key

		// if it's been manually disabled, remove it from the apps list
		if app.Disabled {
			k.Delete(fullKey)
			continue
		}

		// if it's not related to the grafana/loki/tempo stack, move on
		if grafanaEnabled || !slices.Contains(grafanaApps, key) {
			continue
		}

		// if we got this far, it's a grafana related app and we're not using the grafana
		// monitoring stack, so remove it from the config
		k.Delete(fullKey)
	}
}

// setAppDefaults sets default values for application configurations.
// It populates missing fields such as name, provider, organization, branch, and repository URL.
func setAppDefaults(k *koanf.Koanf) {
	p := k.String("providers.source_control")

	var sc schema.GithubConfig
	apps := make(map[string]schema.ApplicationRepositoryConfig)
	k.Unmarshal(p, &sc)
	k.Unmarshal("applications", &apps)

	org := sc.Organization

	for key, app := range apps {
		a := app

		if a.Name == "" {
			a.Name = key
		}

		if a.Provider == "" {
			a.Provider = p
		}

		if a.Organization == "" {
			a.Organization = org
		}

		if a.Branch == "" {
			a.Branch = "main"
		}

		if a.Type == "" {
			s := strings.Split(a.Name, "-")
			if len(s) >= 2 {
				a.Type = s[len(s)-1]
			} else {
				a.Type = "api"
			}
		}

		if a.RepoUrl == "" && !strings.EqualFold(a.Type, "external") {
			a.RepoUrl = githubRepoUrl(a.Organization, a.Name)
		}

		if a.Settings == nil {
			a.Settings = make(map[string]interface{})
		}

		k2 := koanf.New(".")
		k2.Load(structs.Provider(a, "koanf"), nil)

		k.MergeAt(k2, "applications."+key)
	}
}

// setAuthDefaults sets default values for authentication configuration.
// It handles user and group settings, including bulk user creation.
func setAuthDefaults(k *koanf.Koanf) {
	var auth schema.AuthConfig
	k.Unmarshal("auth", &auth)

	domain := k.Get("dns.domain")

	appEnvs := k.MapKeys("environments")

	for username, user := range auth.Users {
		if user.Disabled {
			// remove disabled user from config
			delete(auth.Users, username)
			k.Delete("auth.users." + username)
			continue
		}

		log.Debug("Checking auth user settings", "key", username, "value", user)
		if len(user.Environments) == 0 {
			log.Debug("Setting user env defaults", "user", username, "envs", appEnvs)
			user.Environments = appEnvs
		}

		auth.Users[username] = user

		if user.Count <= 1 {
			// regular user, move on
			continue
		}

		// handle bulk creation of users, replace root with count copies
		for c := range user.Count {
			suffix := fmt.Sprintf("%d", c+1)

			newUsername := username + suffix
			newUser := user // shallow copy

			if newUser.EmailAddress == "" {
				newUser.EmailAddress = fmt.Sprintf("%s%s+%s@%s", newUser.FirstName, newUser.LastName, suffix, domain)
			} else {
				es := strings.Split(newUser.EmailAddress, "@")
				newUser.EmailAddress = fmt.Sprintf("%s+%s@%s", es[0], suffix, es[1])
			}

			newUser.Count = 0
			newUser.LastName = newUser.LastName + suffix

			auth.Users[newUsername] = newUser
		}

		// remove copied user from config
		delete(auth.Users, username)
		k.Delete("auth.users." + username)
	}

	for groupname, group := range auth.Groups {
		log.Debug("Checking auth group settings", "key", groupname, "value", group)
		if len(group.Roles) == 0 {
			log.Debug("Setting group role defaults", "user", groupname, "roles", []string{groupname})
			group.Roles = []string{groupname}
		}

		if len(group.Environments) == 0 {
			log.Debug("Setting group env defaults", "user", groupname, "envs", appEnvs)
			group.Environments = appEnvs
		}

		auth.Groups[groupname] = group
	}

	log.Debug("Updating auth defaults", "value", auth)

	k2 := koanf.New(".")
	k2.Load(structs.Provider(auth, "koanf"), nil)

	k.MergeAt(k2, "auth")
}

// loadStages parses stage configurations from directories and `stage.yaml` files.
// It merges the parsed stages with the existing configuration.
func loadStages(k *koanf.Koanf) {
	stg := make(map[string]schema.StageConfig)
	err := k.Unmarshal("stages", &stg)
	if err != nil {
		log.Debug("No stage overrides found or invalid formatting", "err", err)
	}

	// parse stages from convention directories and discovered stage.yaml configs
	ss := stages.LoadStages(stg, k.Strings("stage_paths")...)
	for key, s := range ss {
		if s.Disabled {
			// TODO: this won't work if a dependent stage is disabled, need something more robust
			k.Delete("stages." + key)
			continue
		}

		k2 := koanf.New(".")
		k2.Load(structs.Provider(s, "koanf"), nil)

		k.MergeAt(k2, "stages."+key)
	}
}

// initTmpDir initializes the temporary directory for Quartz operations.
// It creates the directory if it does not exist and updates the configuration with the directory path.
func initTmpDir(k *koanf.Koanf) (string, error) {
	tmp := k.String("tmp")
	if tmp != "" {
		a, err := filepath.Abs(tmp)
		if err != nil {
			return "", err
		}

		err = os.MkdirAll(a, 0700)
		if err != nil {
			return "", err
		}

		log.Info("Using tmp", "dir", a)
		return a, nil
	}

	name := k.String("name")
	tmp = filepath.Join(os.TempDir(), "quartz", name)
	err := os.MkdirAll(tmp, 0700)
	if err != nil {
		return "", err
	}

	k.Set("tmp", tmp)
	return tmp, nil
}

// githubRepoUrl constructs the GitHub repository URL for the specified organization and repository name.
func githubRepoUrl(org string, name string) string {
	return fmt.Sprintf("https://github.com/%s/%s", org, name)
}
