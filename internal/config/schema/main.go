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

// Package schema defines all config types representing
// the processed quartz.yaml
package schema

import (
	"cmp"
	"path/filepath"
	"slices"

	"github.com/MetroStar/quartzctl/internal/log"
)

// QuartzConfig represents the root configuration struct of the Quartz framework.
type QuartzConfig struct {
	Name    string `koanf:"name"`
	Project string `koanf:"project"`

	Tmp   string      `koanf:"tmp"`
	Chart ChartConfig `koanf:"chart"`

	Providers ProvidersConfig `koanf:"providers"`
	Auth      AuthConfig      `koanf:"auth"`
	Dns       DnsConfig       `koanf:"dns"`

	Aws    AwsConfig    `koanf:"aws"`
	Github GithubConfig `koanf:"github"`

	Core         InfrastructureEnvironmentConfig         `koanf:"core"`
	Environments map[string]ApplicationEnvironmentConfig `koanf:"environments"`
	Alerts       AlertsConfig                            `koanf:"alerts"`

	Kubernetes KubernetesConfig `koanf:"kubernetes"`
	Terraform  TerraformConfig  `koanf:"terraform"`

	StagePaths []string               `koanf:"stage_paths"`
	Stages     map[string]StageConfig `koanf:"stages"`

	Administrators []string                               `koanf:"administrators"`
	Gitops         GitopsConfig                           `koanf:"gitops"`
	Applications   map[string]ApplicationRepositoryConfig `koanf:"applications"`
	Mirror         MirrorConfig                           `koanf:"mirror"`

	Export ExportConfig `koanf:"export"`
	State  StateConfig  `koanf:"state"`

	Log log.LogOptionsConfig `koanf:"log"`

	Internal InternalConfig `koanf:"__internal__"`
}

// QuartzSecrets contains credentials intended to be kept isolated from the QuartzConfig
// to avoid unintentional exposure when serialized/logged.
type QuartzSecrets struct {
	Ironbank   IronbankCredentials   `koanf:"ironbank"`
	Github     GithubCredentials     `koanf:"github"`
	Cloudflare CloudflareCredentials `koanf:"cloudflare"`
}

// StagesOrdered sorts the configured stages map and returns an ordered slice.
func (c *QuartzConfig) StagesOrdered() []StageConfig {
	var r []StageConfig

	for _, v := range c.Stages {
		if v.Manual {
			// leave manual stages out of the ordered list so they're
			// not included in install/clean
			continue
		}
		r = append(r, v)
	}

	slices.SortStableFunc(r, func(x, y StageConfig) int {
		return cmp.Compare(x.Order, y.Order)
	})

	return r
}

// KubeconfigPath derives the expected kubeconfig path based on optional overrides in QuartzConfig.
func (c QuartzConfig) KubeconfigPath() string {
	if len(c.Kubernetes.KubeconfigPath) > 0 {
		p, _ := filepath.Abs(c.Kubernetes.KubeconfigPath)
		return p
	}

	p, _ := filepath.Abs(filepath.Join(c.Tmp, "kubeconfig"))
	return p
}

// TfVarFilePath derives the expected Terraform tfvars path based on optional overrides in QuartzConfig.
func (c QuartzConfig) TfVarFilePath() string {
	p, _ := filepath.Abs(filepath.Join(c.Tmp, "quartz.tfvars.json"))
	return p
}
