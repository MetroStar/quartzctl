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

// GitopsConfig represents the configuration for GitOps in Quartz.
type GitopsConfig struct {
	Core RepositoryConfig `koanf:"core"`
	Apps RepositoryConfig `koanf:"apps"`
}

// DefaultGitopsConfig returns a new GitopsConfig instance with default values.
func DefaultGitopsConfig(p string) GitopsConfig {
	return GitopsConfig{
		Core: RepositoryConfig{
			Name:         "quartz",
			Provider:     p,
			Organization: "",
			Branch:       "main",
		},
		Apps: RepositoryConfig{
			Name:         "quartz-cicd",
			Provider:     p,
			Organization: "",
			Branch:       "", // will be updated to cluster name if not set elsewhere
		},
	}
}
