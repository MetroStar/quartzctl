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

// ExportConfig represents the configuration for exporting resources in Quartz.
type ExportConfig struct {
	Path        string               `koanf:"path"`
	Annotations map[string]string    `koanf:"annotations"`
	Objects     []ExportObjectConfig `koanf:"objects"`
}

// ExportObjectConfig represents the configuration for an individual object to export.
type ExportObjectConfig struct {
	Kind      string `koanf:"kind"`
	Name      string `koanf:"name"`
	Namespace string `koanf:"namespace"`
}

// NewExportConfig returns a new ExportConfig instance with default values.
func NewExportConfig() ExportConfig {
	return ExportConfig{
		Path:        "./backup",
		Annotations: map[string]string{},
		Objects:     []ExportObjectConfig{},
	}
}
