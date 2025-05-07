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

package stages

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/MetroStar/quartzctl/internal/config/schema"
	"github.com/MetroStar/quartzctl/internal/log"
	"github.com/MetroStar/quartzctl/internal/util"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
)

const configFileName = "stage.yaml"

// NewStageConfig creates a new StageConfig with default values for the specified stage ID.
func NewStageConfig(id string) schema.StageConfig {
	return schema.StageConfig{
		Id:           id,
		Description:  id,
		Path:         "",
		Type:         "terraform", // only type supported for now
		Dependencies: nil,
		Order:        -1,
		Vars:         map[string]schema.StageVarsConfig{},
	}
}

// NewStageConfigPath creates a StageConfig by extracting information from the directory name
// and optionally loading additional configuration from a `stage.yaml` file.
func NewStageConfigPath(path string, dir fs.DirEntry) schema.StageConfig {
	order, id, err := parseStageDirName(dir.Name())
	if err != nil {
		log.Debug("Error parsing stage", "path", path, "err", err)
	}

	base := NewStageConfig(id)
	base.Path = path
	base.Order = order

	configPath := filepath.Join(path, configFileName)
	if _, err := os.Stat(configPath); err != nil {
		log.Debug("No config override found for stage", "path", path)
		return base
	}

	k := koanf.New(".")

	// load defaults
	k.Load(structs.Provider(base, "koanf"), nil)

	// parse yaml
	log.Debug("Loading stage config", "file", configPath)
	if err := k.Load(file.Provider(configPath), yaml.Parser()); err != nil {
		log.Debug("No stage config found", "path", configPath, "err", err)
		return base
	}

	var sc schema.StageConfig
	if err := k.Unmarshal("", &sc); err != nil {
		log.Warn("Unable to process stage config, invalid format", "file", configPath, "err", err)
		return base
	}

	return sc
}

// LoadStages loads stage configurations from the specified paths and merges them with the base configuration.
// It processes dependencies and ensures the correct order of stages.
func LoadStages(base map[string]schema.StageConfig, paths ...string) map[string]schema.StageConfig {
	t := make(map[string]schema.StageConfig)
	t = util.MergeMaps(t, base)

	// go through any stage directories and extract records for contained folders
	for _, p := range paths {
		s, err := processStagePath(p)
		if err != nil {
			continue
		}

		// apply selective overrides of supported keys
		for tk, tv := range t {
			if !util.MapContainsKey(s, tk) {
				continue
			}

			sv := s[tk]
			// TODO: what else should be prioritized for override?
			sv.Manual = tv.Manual
			sv.Disabled = tv.Disabled

			s[tk] = sv
		}

		t = util.MergeMaps(t, s)
	}

	// update stage order based on explicitly defined dependencies
	final, err := processStageDependencies(t)
	if err != nil {
		log.Error("unable to process stage dependencies, fatal error", "err", err)
		panic(err)
	}

	return final
}

// processStagePath processes a single stage path and constructs StageConfigs for valid subdirectories.
// Returns a map of stage IDs to StageConfigs or an error if no valid stages are found.
func processStagePath(root string) (map[string]schema.StageConfig, error) {
	log.Debug("Searching stages", "dir", root)
	aroot, _ := filepath.Abs(root)
	srcDirEntries, err := os.ReadDir(aroot)
	if err != nil {
		return nil, err
	}

	r := make(map[string]schema.StageConfig)

	for _, e := range srcDirEntries {
		log.Debug("Checking src subdirectory", "dir", e.Name())
		if !e.IsDir() {
			continue
		}

		stage := NewStageConfigPath(filepath.Join(aroot, e.Name()), e)
		r[stage.Id] = stage
	}

	if len(r) == 0 {
		return nil, fmt.Errorf("no valid stages found")
	}

	return r, nil
}

// processStageDependencies adjusts the order of stages to ensure that all dependencies precede the dependent stages.
// Returns the updated map of stages or an error if a circular dependency is detected.
func processStageDependencies(stages map[string]schema.StageConfig) (map[string]schema.StageConfig, error) {
	// max times to iterate stages list before giving up
	// possible symptom of circular reference, todo
	countdown := 10

	for {
		done := true

		for k, v := range stages {
			if len(v.Dependencies) == 0 {
				log.Debug("Stage has no explicit dependencies, skipping", "stage", k)
				continue
			}

			max := 0

			for _, d := range v.Dependencies {
				dep, ok := stages[d]
				if !ok {
					log.Error("Dependency not found", "stage", k, "dep", d)
					continue
				}

				log.Debug("Checking stage dependency", "stage", k, "dep", dep)
				if dep.Order > max {
					max = dep.Order
				}
			}

			if max >= v.Order {
				log.Debug("Reordering stage due to higher ordered dependency", "stage", k, "max", max)
				done = false // keep going until no changes are made on a single pass
				v.Order = max + 1
				stages[k] = v
			}
		}

		if done {
			log.Debug("Done processing dependencies, no changes on this pass")
			break
		}

		countdown = countdown - 1
		if countdown <= 0 {
			return nil, fmt.Errorf("failed to process stage dependencies, max iterations reached")
		}
	}

	return stages, nil
}

// parseStageDirName parses a directory name with the `<order>-<id>` convention.
// Returns the order, ID, and an error if the format is invalid.
func parseStageDirName(dir string) (int, string, error) {
	defaultOrder := -1

	prefix, id, found := strings.Cut(dir, "-")
	if !found {
		return defaultOrder, dir, fmt.Errorf("unsupported stage name format")
	}

	order, err := strconv.Atoi(prefix)
	if err != nil {
		return defaultOrder, id, fmt.Errorf("invalid stage name prefix, %w", err)
	}

	return order, id, nil
}
