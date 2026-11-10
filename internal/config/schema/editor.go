package schema

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
)

type editorSchema struct {
	Version              string                   `json:"$schema,omitempty"`
	Title                string                   `json:"title,omitempty"`
	Description          string                   `json:"description,omitempty"`
	Type                 string                   `json:"type,omitempty"`
	Properties           map[string]*editorSchema `json:"properties,omitempty"`
	AdditionalProperties any                      `json:"additionalProperties,omitempty"`
	Items                *editorSchema            `json:"items,omitempty"`
	AnyOf                []*editorSchema          `json:"anyOf,omitempty"`
	MinLength            int                      `json:"minLength,omitempty"`
}

// EditorSchema generates type/name assistance without loading configuration or secrets.
func EditorSchema() ([]byte, error) {
	root, err := editorType(reflect.TypeFor[QuartzConfig]())
	if err != nil {
		return nil, err
	}
	root.Version = "https://json-schema.org/draft/2020-12/schema"
	root.Title = "Quartz configuration"
	root.Description = "Editor assistance for quartz.yaml; runtime defaults, environment overrides and provider checks still apply."
	// Framework OpenTofu extensions also consume untyped top-level configuration.
	root.AdditionalProperties = true
	return json.MarshalIndent(root, "", "  ")
}

func editorType(t reflect.Type) (*editorSchema, error) {
	switch t.Kind() {
	case reflect.Struct:
		result := &editorSchema{Type: "object", Properties: map[string]*editorSchema{}, AdditionalProperties: false}
		for i := range t.NumField() {
			field := t.Field(i)
			name, _, _ := strings.Cut(field.Tag.Get("koanf"), ",")
			if !field.IsExported() || name == "" || name == "-" || name == "__internal__" {
				continue
			}
			property, err := editorType(field.Type)
			if err != nil {
				return nil, fmt.Errorf("%s.%s: %w", t.Name(), name, err)
			}
			result.Properties[name] = property
		}
		if t == reflect.TypeFor[ApplicationRepositoryConfig]() {
			policy, err := editorType(reflect.TypeFor[ApplicationPostDeployConfig]())
			if err != nil {
				return nil, err
			}
			for _, name := range []string{"gate_environments", "trigger_environments"} {
				policy.Properties[name].Items.MinLength = 1
			}
			result.Properties["settings"].Properties = map[string]*editorSchema{
				"post_deploy": {AnyOf: []*editorSchema{{Type: "boolean"}, policy}},
			}
		}
		return result, nil
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return nil, fmt.Errorf("unsupported map key type %s", t.Key())
		}
		value, err := editorType(t.Elem())
		if err != nil {
			return nil, err
		}
		return &editorSchema{Type: "object", AdditionalProperties: value}, nil
	case reflect.Slice, reflect.Array:
		item, err := editorType(t.Elem())
		if err != nil {
			return nil, err
		}
		return &editorSchema{Type: "array", Items: item}, nil
	case reflect.Pointer:
		value, err := editorType(t.Elem())
		if err != nil {
			return nil, err
		}
		return &editorSchema{AnyOf: []*editorSchema{value, {Type: "null"}}}, nil
	case reflect.Interface:
		return &editorSchema{}, nil
	case reflect.String:
		return &editorSchema{Type: "string"}, nil
	case reflect.Bool:
		return &editorSchema{Type: "boolean"}, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return &editorSchema{Type: "integer"}, nil
	case reflect.Float32, reflect.Float64:
		return &editorSchema{Type: "number"}, nil
	default:
		return nil, fmt.Errorf("unsupported configuration type %s", t)
	}
}
