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
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"
)

func TestConfigLoadRawConfig(t *testing.T) {
	tmp := t.TempDir()
	cfgContent := []byte(fmt.Sprintf(`
name: mytest
dns:
  zone: example.com
providers:
  cloud: local
tmp: %s
`, tmp))
	cfgFile := filepath.Join(tmp, "test-config.yaml")
	fmt.Printf("Using %s\n", cfgFile)
	os.WriteFile(cfgFile, cfgContent, 0664)

	t.Setenv("QUARTZ_project", "testproject")

	actual, err := LoadRawConfig(context.Background(), cfgFile)
	if err != nil {
		t.Errorf("failed loading raw config, %v", err)
		return
	}

	expected := map[string]interface{}{
		// from config file
		"name":            "mytest",
		"dns.zone":        "example.com",
		"providers.cloud": "local",
		"tmp":             tmp,

		// from env
		"project": "testproject",

		// derived
		"dns.domain": "mytest.example.com",

		// defaults
		"tofu.version": "1.12.0",
	}
	for k, v := range expected {
		a := actual.Get(k)
		if v != a {
			t.Errorf("mismatched value found for %s, expected %v, found %v", k, v, a)
		}
	}
}

func TestConfigLoadRawSecrets(t *testing.T) {
	tmp := t.TempDir()
	cfgContent := []byte(`
github:
  username: test-github-user
  token: super-secret-github-token
`)
	cfgFile := filepath.Join(tmp, "test-secrets.yaml")
	fmt.Printf("Using %s\n", cfgFile)
	os.WriteFile(cfgFile, cfgContent, 0664)

	t.Setenv("REGISTRY_USERNAME", "test-ironbank-user")
	t.Setenv("REGISTRY_PASSWORD", "")

	actual, err := LoadRawSecrets(context.Background(), cfgFile)
	if err != nil {
		t.Errorf("failed loading raw secrets, %v", err)
		return
	}

	expected := map[string]interface{}{
		// from secrets file
		"github.username": "test-github-user",
		"github.token":    "super-secret-github-token",

		// from env
		"ironbank.username": "test-ironbank-user",
		"ironbank.password": nil,
	}
	for k, v := range expected {
		a := actual.Get(k)
		if v != a {
			t.Errorf("mismatched value found for %s, expected %v, found %v", k, v, a)
		}
	}
}

func TestConfigLoad(t *testing.T) {
	tmp := t.TempDir()
	cfgContent := []byte(fmt.Sprintf(`
name: mytest
dns:
  zone: example.com
providers:
  cloud: local
tmp: %s
applications:
  app1:
    repo: example-app1
auth:
  users:
    testuser1: {}
  groups:
    testgroup2: {}
`, tmp))
	cfgFile := filepath.Join(tmp, "test-config.yaml")
	fmt.Printf("Using %s\n", cfgFile)
	os.WriteFile(cfgFile, cfgContent, 0664)

	t.Setenv("QUARTZ_project", "testproject")
	t.Setenv("REGISTRY_USERNAME", "test-ironbank-user")
	t.Setenv("REGISTRY_PASSWORD", "")

	actual, err := Load(context.Background(), cfgFile, "")
	if err != nil {
		t.Errorf("failed loading config, %v", err)
		return
	}

	if actual.Config.Name != "mytest" ||
		actual.Config.Dns.Zone != "example.com" ||
		actual.Config.Dns.Domain != "mytest.example.com" ||
		actual.Config.Providers.Cloud != "local" ||
		actual.Config.Tmp != tmp ||
		actual.Config.Project != "testproject" ||
		actual.Config.Tofu.Version != "1.12.0" {
		t.Errorf("mismatched config value found, expected %s, found %v", cfgContent, actual.Config)
	}

	if actual.Secrets.Ironbank.Username != "test-ironbank-user" ||
		actual.Secrets.Ironbank.Password != "" {
		t.Errorf("mismatched secret value found, found %v", actual.Secrets)
	}

	if actual.rawConfig == nil ||
		actual.rawSecrets == nil {
		t.Errorf("missing raw config objects on result")
	}
}

func TestConfigParseDnsZone(t *testing.T) {
	tmp := t.TempDir()
	cfgContent := []byte(fmt.Sprintf(`
name: mytest
dns:
  domain: foobar.example.com
providers:
  cloud: local
tmp: %s
`, tmp))
	cfgFile := filepath.Join(tmp, "test-config.yaml")
	fmt.Printf("Using %s\n", cfgFile)
	os.WriteFile(cfgFile, cfgContent, 0664)

	actual, err := Load(context.Background(), cfgFile, "")
	if err != nil {
		t.Errorf("failed loading config, %v", err)
		return
	}

	if actual.Config.Name != "mytest" ||
		actual.Config.Dns.Zone != "example.com" ||
		actual.Config.Dns.Domain != "foobar.example.com" {
		t.Errorf("mismatched config value found, expected %s, found %v", cfgContent, actual.Config)
	}
}

func TestConfigFailOnMissingDns(t *testing.T) {
	tmp := t.TempDir()
	cfgContent := []byte(fmt.Sprintf(`
name: mytest
dns: {}
providers:
  cloud: local
tmp: %s
`, tmp))
	cfgFile := filepath.Join(tmp, "test-config.yaml")
	fmt.Printf("Using %s\n", cfgFile)
	os.WriteFile(cfgFile, cfgContent, 0664)

	actual, err := Load(context.Background(), cfgFile, "")
	if err == nil {
		t.Errorf("expected error, found %v", actual)
		return
	}
}

func TestConfigStagesOrdered(t *testing.T) {
	tmp := t.TempDir()
	cfgContent := []byte(fmt.Sprintf(`
name: mytest
dns:
  zone: example.com
providers:
  cloud: local
tmp: %s
stages:
  0:
    id: last_stage
    order: 999
  1:
    id: first_stage
    order: 1
  2:
    id: middle_stage
    order: 10
`, tmp))
	cfgFile := filepath.Join(tmp, "test-config.yaml")
	fmt.Printf("Using %s\n", cfgFile)
	os.WriteFile(cfgFile, cfgContent, 0664)

	conf, err := Load(context.Background(), cfgFile, "")
	if err != nil {
		t.Errorf("failed loading config, %v", err)
		return
	}

	actual := conf.Config.StagesOrdered()
	if len(actual) != 3 ||
		actual[0].Id != "first_stage" ||
		actual[1].Id != "middle_stage" ||
		actual[2].Id != "last_stage" {
		t.Errorf("incorrect stage ordering, found %v", actual)
	}
}

func TestConfigKubeconfigPathDefault(t *testing.T) {
	tmp := t.TempDir()
	cfgContent := []byte(fmt.Sprintf(`
name: mytest
dns:
  zone: example.com
providers:
  cloud: local
tmp: %s
`, tmp))
	cfgFile := filepath.Join(tmp, "test-config.yaml")
	fmt.Printf("Using %s\n", cfgFile)
	os.WriteFile(cfgFile, cfgContent, 0664)

	conf, err := Load(context.Background(), cfgFile, "")
	if err != nil {
		t.Errorf("failed loading config, %v", err)
		return
	}

	actual := conf.Config.KubeconfigPath()
	if actual != path.Join(tmp, "kubeconfig") {
		t.Errorf("incorrect kubeconfig path, found %v", actual)
	}
}

func TestConfigApplicationDefaults(t *testing.T) {
	tmp := t.TempDir()
	cfgContent := []byte(fmt.Sprintf(`
name: mytest
dns:
  zone: example.com
providers:
  cloud: local
tmp: %s
applications:
  app1:
    settings: {}
  app2:
    settings:
      post_deploy: false
  app3:
    path: custom/deploy
    settings:
      post_deploy:
        trigger_environments:
        - dev
`, tmp))
	cfgFile := filepath.Join(tmp, "test-config.yaml")
	os.WriteFile(cfgFile, cfgContent, 0664)

	actual, err := Load(context.Background(), cfgFile, "")
	if err != nil {
		t.Fatalf("failed loading config, %v", err)
	}

	app1 := actual.Config.Applications["app1"]
	if app1.Path != "deploy" {
		t.Fatalf("expected default app path deploy, found %q", app1.Path)
	}

	app1PostDeploy, ok := app1.Settings["post_deploy"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected typed post_deploy defaults in app1 settings, found %#v", app1.Settings["post_deploy"])
	}
	if enabled, ok := app1PostDeploy["enabled"].(bool); !ok || !enabled {
		t.Fatalf("expected app1 post_deploy.enabled=true, found %#v", app1PostDeploy["enabled"])
	}

	app2PostDeploy := actual.Config.Applications["app2"].Settings["post_deploy"].(map[string]interface{})
	if enabled, ok := app2PostDeploy["enabled"].(bool); !ok || enabled {
		t.Fatalf("expected app2 post_deploy.enabled=false, found %#v", app2PostDeploy["enabled"])
	}

	app3 := actual.Config.Applications["app3"]
	if app3.Path != "custom/deploy" {
		t.Fatalf("expected custom app path to be preserved, found %q", app3.Path)
	}
	app3PostDeploy := app3.Settings["post_deploy"].(map[string]interface{})
	trigger, ok := app3PostDeploy["trigger_environments"].([]string)
	if !ok || len(trigger) != 1 || trigger[0] != "dev" {
		t.Fatalf("expected app3 trigger_environments=[dev], found %#v", app3PostDeploy["trigger_environments"])
	}
	gate, ok := app3PostDeploy["gate_environments"].([]string)
	if !ok || len(gate) != 2 || gate[0] != "stage" || gate[1] != "prod" {
		t.Fatalf("expected default gate environments, found %#v", app3PostDeploy["gate_environments"])
	}
}

func TestConfigGitopsAppsOptional(t *testing.T) {
	tmp := t.TempDir()
	cfgContent := []byte(fmt.Sprintf(`
name: mytest
dns:
  zone: example.com
providers:
  cloud: local
tmp: %s
gitops:
  core:
    repo: quartz
applications:
  app1:
    repo_url: https://github.com/example/app1.git
`, tmp))
	cfgFile := filepath.Join(tmp, "test-config.yaml")
	os.WriteFile(cfgFile, cfgContent, 0664)

	actual, err := Load(context.Background(), cfgFile, "")
	if err != nil {
		t.Fatalf("failed loading config, %v", err)
	}

	if actual.Config.Gitops.Apps.Name != "" || actual.Config.Gitops.Apps.RepoUrl != "" || actual.Config.Gitops.Apps.Branch != "" {
		t.Fatalf("expected gitops.apps to remain optional/empty, found %#v", actual.Config.Gitops.Apps)
	}
}

func TestConfigKubeconfigPathOverride(t *testing.T) {
	tmp := t.TempDir()
	cfgContent := []byte(fmt.Sprintf(`
name: mytest
dns:
  zone: example.com
providers:
  cloud: local
tmp: %s
kubernetes:
  kubeconfig_path: mykubeconfig.override
`, tmp))
	cfgFile := filepath.Join(tmp, "test-config.yaml")
	fmt.Printf("Using %s\n", cfgFile)
	os.WriteFile(cfgFile, cfgContent, 0664)

	conf, err := Load(context.Background(), cfgFile, "")
	if err != nil {
		t.Errorf("failed loading config, %v", err)
		return
	}

	actual := conf.Config.KubeconfigPath()
	pwd, _ := os.Getwd()
	if actual != path.Join(pwd, "mykubeconfig.override") {
		t.Errorf("incorrect kubeconfig path, found %v", actual)
	}
}
