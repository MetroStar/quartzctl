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

package config

import (
	jsonenc "encoding/json"

	"github.com/MetroStar/quartzctl/internal/config/schema"
	"github.com/MetroStar/quartzctl/internal/log"
	"github.com/MetroStar/quartzctl/internal/util"
	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/v2"
)

// Settings is a wrapper for the main Quartz configuration and secrets.
type Settings struct {
	Config  schema.QuartzConfig  // The parsed Quartz configuration.
	Secrets schema.QuartzSecrets // The parsed Quartz secrets.

	rawConfig  *koanf.Koanf // The raw configuration data.
	rawSecrets *koanf.Koanf // The raw secrets data.
}

// NewSettings creates a new Settings instance by parsing the provided configuration and secrets maps.
// Returns an error if unmarshalling fails.
func NewSettings(c *koanf.Koanf, s *koanf.Koanf) (Settings, error) {
	var config schema.QuartzConfig
	if err := c.Unmarshal("", &config); err != nil {
		return Settings{}, err
	}

	var secrets schema.QuartzSecrets
	if err := s.Unmarshal("", &secrets); err != nil {
		return Settings{}, err
	}

	return Settings{
		rawConfig:  c,
		rawSecrets: s,

		Config:  config,
		Secrets: secrets,
	}, nil
}

// ConfigString retrieves a raw configuration value by its key.
func (r Settings) ConfigString(key string) string {
	return r.rawConfig.String(key)
}

// SecretString retrieves a raw secret value by its key.
func (r Settings) SecretString(key string) string {
	return r.rawSecrets.String(key)
}

// WriteJsonConfig writes the application configuration to a JSON file.
// Supports an optional root key and indentation for pretty printing.
func (r Settings) WriteJsonConfig(path string, root string, indent bool) error {
	b, err := marshalJsonRoot(r.rawConfig, root, indent)
	if err != nil {
		return err
	}

	log.Info("Writing JSON config", "path", path)
	return util.WriteBytesToFile(b, path)
}

// WriteYamlConfig writes the application configuration to a YAML file.
func (r Settings) WriteYamlConfig(path string) error {
	parser := yaml.Parser()

	b, err := r.rawConfig.Marshal(parser)
	if err != nil {
		return err
	}

	log.Info("Writing YAML config", "path", path)
	return util.WriteBytesToFile(b, path)
}

// marshalJsonRoot serializes the configuration to JSON, optionally nesting it under a root key.
// Supports indentation for pretty printing.
func marshalJsonRoot(k *koanf.Koanf, root string, indent bool) ([]byte, error) {
	if root == "" {
		// just serialize to json normally
		return marshalJson(k, indent)
	}

	// nest the config under a root key
	copy := koanf.New(".")
	copy.MergeAt(k, root)

	return marshalJson(copy, indent)
}

// marshalJson serializes the configuration to JSON.
// Supports indentation for pretty printing.
func marshalJson(k *koanf.Koanf, indent bool) ([]byte, error) {
	parser := json.Parser()

	if !indent {
		return k.Marshal(parser)
	}

	// unflatten and pretty print
	b, _ := k.Marshal(parser)
	var tmp map[string]interface{}
	err := jsonenc.Unmarshal(b, &tmp)
	if err != nil {
		return nil, err
	}

	return jsonenc.MarshalIndent(tmp, "", "  ")
}
