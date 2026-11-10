package schema

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEditorSchemaIsStableAndContainsTypesNotRuntimeValues(t *testing.T) {
	first, err := EditorSchema()
	require.NoError(t, err)
	second, err := EditorSchema()
	require.NoError(t, err)
	require.Equal(t, first, second)
	var document editorSchema
	require.NoError(t, json.Unmarshal(first, &document))
	require.Equal(t, "https://json-schema.org/draft/2020-12/schema", document.Version)
	require.Equal(t, "object", document.Type)
	require.Equal(t, true, document.AdditionalProperties)
	require.NotContains(t, document.Properties, "__internal__")
	require.NotContains(t, document.Properties, "ironbank")
	require.NotContains(t, document.Properties, "cloudflare")
	require.Equal(t, "string", document.Properties["name"].Type)
	require.Equal(t, "string", document.Properties["aws"].Properties["region"].Type)
	require.NotContains(t, string(first), `"default"`)
	require.NotContains(t, string(first), "example.com")
}

func TestEditorSchemaKeepsExtensionsOpenAndPostDeployTyped(t *testing.T) {
	root, err := editorType(reflect.TypeFor[QuartzConfig]())
	require.NoError(t, err)
	environment := root.Properties["environments"].AdditionalProperties.(*editorSchema)
	require.Equal(t, "boolean", environment.Properties["enabled"].Type)
	application := root.Properties["applications"].AdditionalProperties.(*editorSchema)
	require.Equal(t, false, application.AdditionalProperties)
	require.Equal(t, "string", application.Properties["callback_urls"].Items.Properties["url"].Type)
	require.Equal(t, &editorSchema{}, application.Properties["keycloak"].AdditionalProperties)
	settings := application.Properties["settings"]
	require.Equal(t, &editorSchema{}, settings.AdditionalProperties)
	policy := settings.Properties["post_deploy"].AnyOf
	require.Len(t, policy, 2)
	require.Equal(t, "boolean", policy[0].Type)
	require.Equal(t, "object", policy[1].Type)
	require.Equal(t, "boolean", policy[1].Properties["enabled"].Type)
	require.Equal(t, "boolean", policy[1].Properties["require_report"].Type)
	require.Equal(t, "string", policy[1].Properties["health_url"].Type)
	require.Equal(t, "integer", policy[1].Properties["expected_status"].Type)
	require.Equal(t, "string", policy[1].Properties["headers"].AdditionalProperties.(*editorSchema).Type)
	require.Equal(t, "string", policy[1].Properties["gate_environments"].Items.Type)
	require.Equal(t, 1, policy[1].Properties["trigger_environments"].Items.MinLength)
	require.Equal(t, false, policy[1].AdditionalProperties)
	stage := root.Properties["stages"].AdditionalProperties.(*editorSchema)
	check := stage.Properties["checks"].AdditionalProperties.(*editorSchema)
	wait := check.Properties["kubernetes"].Items.Properties["wait"]
	require.Equal(t, "boolean", wait.AnyOf[0].Type)
	require.Equal(t, "null", wait.AnyOf[1].Type)
}

func TestEditorSchemaUsesKoanfNamesAndFailsOnUnsupportedTypes(t *testing.T) {
	type input struct {
		Value   string `koanf:"value"`
		Skip    string `koanf:"-"`
		NoTag   string
		private string `koanf:"private"`
	}
	require.Empty(t, input{}.private)
	result, err := editorType(reflect.TypeFor[input]())
	require.NoError(t, err)
	require.Len(t, result.Properties, 1)
	require.Contains(t, result.Properties, "value")
	for _, invalid := range []reflect.Type{
		reflect.TypeFor[chan string](),
		reflect.TypeFor[map[int]string](),
		reflect.TypeFor[struct {
			Bad chan int `koanf:"bad"`
		}](),
	} {
		_, err := editorType(invalid)
		require.ErrorContains(t, err, "unsupported")
	}
}
