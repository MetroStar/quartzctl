package config

import (
	"reflect"
	"testing"

	"github.com/MetroStar/quartzctl/internal/config/schema"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
	"github.com/stretchr/testify/require"
)

func TestRedactedYAMLPreservesDefaultsAndMasksOpaqueValues(t *testing.T) {
	k := koanf.New(".")
	require.NoError(t, k.Load(file.Provider(writeInputConfig(t, `
name: inspection
providers:
  cloud: local
auth:
  users:
    fixture:
      password:
        value: fixture-private-password
      groups: [readers]
applications:
  api:
    repo_url: https://fixture-user:fixture-private-url@example.invalid/api?token=fixture-private-query#fixture-private-fragment
    settings:
      custom: {nested: fixture-private-custom}
      numeric_pin: 123456
      post_deploy:
        enabled: true
        require_report: true
        gate_environments: [stage, prod]
        health_path: /ready?key=fixture-private-relative#fixture-private-anchor
        expected_status: 200
        headers: {X-Api-Key: fixture-private-header}
        custom: fixture-private-policy
    keycloak: {custom: fixture-private-keycloak}
stages:
  local:
    order: 1
    vars:
      fixture:
        value: fixture-private-stage
    checks:
      ready:
        kubernetes:
          - name: api
            wait: false
export:
  annotations: {custom: fixture-private-annotation}
security: {custom: fixture-private-extension}
__internal__: {custom: fixture-private-internal}
`)), yaml.Parser()))
	before := k.Raw()
	data, err := RedactedYAML(k)
	require.NoError(t, err)
	require.NotContains(t, string(data), "fixture-private")
	require.NotContains(t, string(data), "fixture-user")
	require.NotContains(t, string(data), "123456")
	require.Equal(t, before, k.Raw(), "redaction must not modify deployment input")
	out, err := yaml.Parser().Unmarshal(data)
	require.NoError(t, err)
	actual := koanf.New(".")
	require.NoError(t, actual.Load(file.Provider(writeInputConfig(t, string(data))), yaml.Parser()))
	require.Equal(t, "inspection", out["name"])
	require.Equal(t, "https://example.invalid/api", actual.String("applications.api.repo_url"))
	require.Equal(t, "/ready", actual.String("applications.api.settings.post_deploy.health_path"))
	require.True(t, actual.Bool("applications.api.settings.post_deploy.require_report"))
	require.Equal(t, []string{"stage", "prod"}, actual.Strings("applications.api.settings.post_deploy.gate_environments"))
	require.Equal(t, 200, actual.Int("applications.api.settings.post_deploy.expected_status"))
	require.Equal(t, redacted, actual.String("auth.users.fixture.password"))
	require.Equal(t, redacted, actual.String("applications.api.settings.custom"))
	require.Equal(t, redacted, actual.String("applications.api.settings.post_deploy.headers"))
	require.Equal(t, redacted, actual.String("applications.api.settings.post_deploy.custom"))
	require.Equal(t, redacted, actual.String("__internal__"))
	require.Equal(t, redacted, actual.String("security"))
	require.Equal(t, 1, actual.Int("stages.local.order"))
}

func TestRedactConfigRejectsMismatchedTypesAndInvalidURLs(t *testing.T) {
	for _, test := range []struct {
		value interface{}
		typ   reflect.Type
		want  interface{}
	}{
		{nil, reflect.TypeFor[string](), nil},
		{"fixture-private", reflect.TypeFor[bool](), redacted},
		{"fixture-private", reflect.TypeFor[int](), redacted},
		{"fixture-private", reflect.TypeFor[[]string](), redacted},
		{"fixture-private", reflect.TypeFor[schema.DnsConfig](), redacted},
		{"fixture-private", reflect.TypeFor[map[string]string](), redacted},
		{123456, reflect.TypeFor[string](), redacted},
		{"https://example.invalid/%zz?fixture-private", reflect.TypeFor[string](), redacted},
		{"https:///fixture-private", reflect.TypeFor[string](), redacted},
		{false, reflect.TypeFor[*bool](), false},
		{[]string{"stage", "prod"}, reflect.TypeFor[[]string](), []interface{}{"stage", "prod"}},
		{true, reflect.TypeFor[interface{}](), redacted},
		{map[string]interface{}{"enabled": "fixture-private"}, reflect.TypeFor[schema.ApplicationPostDeployConfig](), map[string]interface{}{"enabled": redacted}},
	} {
		require.Equal(t, test.want, redactConfig(test.value, test.typ))
	}
	require.Equal(t, false, redactApplicationSettings(map[string]interface{}{"post_deploy": false}).(map[string]interface{})["post_deploy"])
	require.Equal(t, redacted, redactApplicationSettings("fixture-private"))
}
