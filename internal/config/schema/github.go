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

// GithubConfig represents the configuration for GitHub integration.
type GithubConfig struct {
	TagReleaseEnabled bool           `koanf:"tag_release"`
	Webhooks          GithubWebhooks `koanf:"webhooks"`
	Organization      string         `koanf:"organization"`
}

// GithubCredentials represents the credentials for accessing GitHub.
type GithubCredentials struct {
	Username string `koanf:"username"`
	Token    string `koanf:"token"`
}

// GithubWebhooks represents the configuration for GitHub webhooks.
type GithubWebhooks struct {
	Build   bool `koanf:"build"`
	Release bool `koanf:"release"`
}

// NewGithubConfig returns a new GithubConfig instance with default values.
func NewGithubConfig() GithubConfig {
	return GithubConfig{
		Organization: "MetroStar",
		Webhooks: GithubWebhooks{
			Build:   true,
			Release: false,
		},
		TagReleaseEnabled: false,
	}
}
