package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/knadh/koanf/parsers/json"
)

func writeInputConfig(t *testing.T, body string) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "quartz.yaml")
	if err := os.WriteFile(file, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestConfigRejectsUnreadableOrMalformedInput(t *testing.T) {
	t.Setenv("QUARTZ_PROVIDERS_CLOUD", "local")
	t.Setenv("QUARTZ_DNS_ZONE", "example.com")
	for _, file := range []string{
		filepath.Join(t.TempDir(), "absent.yaml"),
		writeInputConfig(t, "name: [unterminated"),
	} {
		_, err := LoadRawConfig(context.Background(), file)
		if err == nil || !strings.Contains(err.Error(), "load configuration") {
			t.Fatalf("expected explicit configuration error for %s, got %v", file, err)
		}
	}
}

func TestConfigOverridesPrecedeDependentDefaults(t *testing.T) {
	file := writeInputConfig(t, `
name: original
providers:
  cloud: local
dns:
  zone: original.example
github:
  organization: original
applications:
  service-api: {}
gitops:
  core:
    repo: infrastructure
`)
	t.Setenv("QUARTZ_NAME", "override")
	t.Setenv("QUARTZ_DNS_ZONE", "override.example")
	t.Setenv("QUARTZ_GITHUB_ORGANIZATION", "override-org")
	t.Setenv("QUARTZ_TMP", t.TempDir())
	config, err := LoadRawConfig(context.Background(), file)
	if err != nil {
		t.Fatal(err)
	}
	for key, expected := range map[string]string{
		"dns.domain":                        "override.override.example",
		"applications.service-api.repo_url": "https://github.com/override-org/service-api",
		"gitops.core.repo_url":              "https://github.com/override-org/infrastructure",
	} {
		if actual := config.String(key); actual != expected {
			t.Errorf("%s: expected %q, got %q", key, expected, actual)
		}
	}
}

func TestConfigExplicitDomainAndTemporaryDirectoryErrors(t *testing.T) {
	t.Run("explicit domain preserved", func(t *testing.T) {
		file := writeInputConfig(t, `
name: cluster
providers:
  cloud: local
dns:
  domain: custom.example.com
  zone: example.com
`)
		t.Setenv("QUARTZ_TMP", t.TempDir())
		config, err := LoadRawConfig(context.Background(), file)
		if err != nil {
			t.Fatal(err)
		}

		if actual := config.String("dns.domain"); actual != "custom.example.com" {
			t.Fatalf("explicit domain overwritten: %q", actual)
		}
	})
	t.Run("invalid bare domain rejected", func(t *testing.T) {
		file := writeInputConfig(t, "providers:\n  cloud: local\ndns:\n  domain: localhost\n")
		_, err := LoadRawConfig(context.Background(), file)
		if err == nil || !strings.Contains(err.Error(), "dns.domain") {
			t.Fatalf("expected dns.domain error, got %v", err)
		}
	})
	t.Run("temporary directory failure rejected", func(t *testing.T) {
		file := writeInputConfig(t, "providers:\n  cloud: local\ndns:\n  zone: example.com\n")
		t.Setenv("QUARTZ_TMP", file)
		_, err := LoadRawConfig(context.Background(), file)
		if err == nil || !strings.Contains(err.Error(), "temporary directory") {
			t.Fatalf("expected temporary directory error, got %v", err)
		}
	})
}

func TestConfigPostDeployExplicitEmptyAndInvalidSettings(t *testing.T) {
	for _, test := range []struct {
		name      string
		input     string
		wantError string
	}{
		{"empty overrides", "post_deploy:\n        enabled: false\n        gate_environments: []\n        trigger_environments: []", ""},
		{"invalid enabled", "post_deploy:\n        enabled: invalid", "enabled must be a boolean"},
		{"invalid report flag", "post_deploy:\n        require_report: invalid", "require_report must be a boolean"},
		{"invalid environment list", "post_deploy:\n        gate_environments: stage", "gate_environments"},
		{"invalid list entry", "post_deploy:\n        trigger_environments: [dev, false]", "trigger_environments"},
		{"empty list entry", "post_deploy:\n        trigger_environments: ['']", "trigger_environments"},
		{"invalid policy", "post_deploy: invalid", "must be a boolean or mapping"},
	} {
		t.Run(test.name, func(t *testing.T) {
			file := writeInputConfig(t, "providers:\n  cloud: local\ndns:\n  zone: example.com\napplications:\n  app:\n    settings:\n      "+test.input+"\n")
			t.Setenv("QUARTZ_TMP", t.TempDir())
			config, err := LoadRawConfig(context.Background(), file)
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) || !strings.Contains(err.Error(), "applications.app.settings.post_deploy") {
					t.Fatalf("expected field-specific %q error, got %v", test.wantError, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if config.Bool("applications.app.settings.post_deploy.enabled") ||
				len(config.Strings("applications.app.settings.post_deploy.gate_environments")) != 0 ||
				len(config.Strings("applications.app.settings.post_deploy.trigger_environments")) != 0 {
				t.Fatal("explicit false/empty policy was replaced with defaults")
			}
		})
	}
}

func TestConfigCompactApplicationMatchesExplicitDefaults(t *testing.T) {
	t.Setenv("QUARTZ_TMP", t.TempDir())
	const base = `
providers:
  cloud: local
dns:
  zone: example.com
github:
  organization: example
applications:
  service-api:
    type: api
    base_url: https://api-$env.$domain
    db:
      enabled: true
`
	compact, err := LoadRawConfig(context.Background(), writeInputConfig(t, base))
	if err != nil {
		t.Fatal(err)
	}

	explicit, err := LoadRawConfig(context.Background(), writeInputConfig(t, base+`
    repo: service-api
    repo_url: https://github.com/example/service-api
    provider: github
    organization: example
    branch: main
    path: deploy
    settings:
      post_deploy:
        enabled: true
        gate_environments: [stage, prod]
        trigger_environments: [dev, stage, prod]
`))
	if err != nil {
		t.Fatal(err)
	}
	compactJSON, err := compact.Marshal(json.Parser())
	if err != nil {
		t.Fatal(err)
	}
	explicitJSON, err := explicit.Marshal(json.Parser())
	if err != nil {
		t.Fatal(err)
	}
	if string(compactJSON) != string(explicitJSON) {
		t.Fatal("omitting redundant application defaults changed the effective configuration")
	}
}

func TestConfigPreservesRequiredPostDeployReports(t *testing.T) {
	t.Setenv("QUARTZ_TMP", t.TempDir())
	input := writeInputConfig(t, `
providers:
  cloud: local
dns:
  zone: example.com
applications:
  service-api:
    settings:
      post_deploy:
        require_report: true
`)
	config, err := LoadRawConfig(context.Background(), input)
	if err != nil {
		t.Fatal(err)
	}
	if !config.Bool("applications.service-api.settings.post_deploy.require_report") {
		t.Fatal("explicit report requirement was not preserved")
	}
}
