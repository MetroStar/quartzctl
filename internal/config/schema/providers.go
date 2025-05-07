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

package schema

// ProvidersConfig represents the configuration for various providers used in Quartz.
type ProvidersConfig struct {
	Cloud         string `koanf:"cloud"`
	Dns           string `koanf:"dns"`
	SourceControl string `koanf:"source_control"`
	Monitoring    string `koanf:"monitoring"`
	Secrets       string `koanf:"secrets"`
	Oidc          string `koanf:"oidc"`
	CiCd          string `koanf:"cicd"`
}

// NewProvidersConfig returns a new ProvidersConfig instance with default values.
func NewProvidersConfig() ProvidersConfig {
	return ProvidersConfig{
		Cloud:         "aws",
		Dns:           "aws",
		SourceControl: "github",
		Monitoring:    "cloudwatch",
		Secrets:       "aws-ssm-parameter",
		Oidc:          "keycloak",
		CiCd:          "jenkins",
	}
}
