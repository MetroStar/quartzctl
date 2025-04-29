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
