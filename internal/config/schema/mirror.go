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

// MirrorConfig represents the configuration for mirroring resources in Quartz.
type MirrorConfig struct {
	ImageRepository MirrorImageRepositoryConfig `koanf:"image_repository"`
	Grype           bool                        `koanf:"grype"`
}

// MirrorImageRepositoryConfig represents the configuration for mirroring image repositories.
type MirrorImageRepositoryConfig struct {
	Enabled          bool     `koanf:"enabled"`
	Target           string   `koanf:"target"`
	SourceRegistries []string `koanf:"source_registries"`
}

// NewMirrorConfig returns a new MirrorConfig instance with default values.
func NewMirrorConfig() MirrorConfig {
	return MirrorConfig{
		ImageRepository: MirrorImageRepositoryConfig{
			Enabled: true,
			Target:  "ghcr.io/metrostar/quartz-pkgs",
			SourceRegistries: []string{
				"registry1.dso.mil",
				"registry.dso.mil",
				"quay.io",
			},
		},
	}
}
