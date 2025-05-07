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

// RepositoryConfig represents the configuration for a repository.
type RepositoryConfig struct {
	Name         string `koanf:"repo"`
	RepoUrl      string `koanf:"repo_url"`
	Provider     string `koanf:"provider"`
	Organization string `koanf:"organization"`
	Branch       string `koanf:"branch"`
}

// ApplicationRepositoryConfig represents the configuration for an application repository.
type ApplicationRepositoryConfig struct {
	Name         string                      `koanf:"repo"`
	RepoUrl      string                      `koanf:"repo_url"`
	Provider     string                      `koanf:"provider"`
	Organization string                      `koanf:"organization"`
	Branch       string                      `koanf:"branch"`
	Type         string                      `koanf:"type"`
	Db           ApplicationDbConfig         `koanf:"db"`
	BaseUrl      string                      `koanf:"base_url"`
	CallbackUrls []ApplicationCallbackConfig `koanf:"callback_urls"`
	Keycloak     map[string]interface{}      `koanf:"keycloak"`

	// Cloud specific and other schema-less settings for the app
	Settings map[string]interface{} `koanf:"settings"`
}

// ApplicationDbConfig represents the database configuration for an application.
type ApplicationDbConfig struct {
	Enabled  bool   `koanf:"enabled"`
	Admin    bool   `koanf:"admin"`
	Username string `koanf:"username"`
	DbName   string `koanf:"db_name"`
}

// ApplicationCallbackConfig represents the callback configuration for an application.
type ApplicationCallbackConfig struct {
	Url  string `koanf:"url"`
	Path string `koanf:"path"`
}

// RepositoryConfig converts an ApplicationRepositoryConfig to a RepositoryConfig.
func (c ApplicationRepositoryConfig) RepositoryConfig() RepositoryConfig {
	return RepositoryConfig{
		Name:         c.Name,
		RepoUrl:      c.RepoUrl,
		Provider:     c.Provider,
		Organization: c.Organization,
		Branch:       c.Branch,
	}
}
