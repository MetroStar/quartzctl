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

package util

import (
	"github.com/MetroStar/quartzctl/internal/log"
	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
)

// MarshalToJsonBytes marshals a struct of type `T` into JSON-encoded bytes.
func MarshalToJsonBytes[T any](o T) []byte {
	return MarshalKoanfStructToBytes(o, json.Parser())
}

// MarshalToYamlBytes marshals a struct of type `T` into YAML-encoded bytes.
func MarshalToYamlBytes[T any](o T) []byte {
	return MarshalKoanfStructToBytes(o, yaml.Parser())
}

// MarshalKoanfStructToBytes marshals a struct of type `T` into bytes using the specified `koanf.Parser`.
// This function uses the Koanf library to handle the marshaling process.
func MarshalKoanfStructToBytes[T any](o T, p koanf.Parser) []byte {
	k := koanf.New(".")
	if err := k.Load(structs.Provider(o, "koanf"), nil); err != nil {
		log.Warn("Error loading struct", "err", err)
		return nil
	}
	b, _ := k.Marshal(p)
	return b
}
