package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNativeExecutableLifecycle(t *testing.T) {
	binary := os.Getenv("QUARTZ_NATIVE_BINARY")
	if binary == "" {
		t.Skip("set QUARTZ_NATIVE_BINARY to a freshly built native quartz executable")
	}
	binary, err := filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(t.TempDir(), "project with spaces")
	stage := filepath.Join(workspace, "stages", "10-local")
	if err := os.MkdirAll(stage, 0o700); err != nil {
		t.Fatal(err)
	}
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	config := filepath.Join(workspace, "quartz.yaml")
	write(config, `name: native-lifecycle
providers:
  cloud: local
dns:
  zone: example.invalid
tmp: "./runtime with spaces"
tofu:
  version: "1.13.1"
stage_paths: [stages]
log:
  file:
    enabled: false
  tofu:
    enabled: false
`)
	write(filepath.Join(stage, "main.tf"), `resource "terraform_data" "fixture" {
  input = "native-runtime-verified"
}
output "result" {
  value = terraform_data.fixture.output
}
`)
	env := []string{}
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(strings.ToUpper(name), "QUARTZ_") ||
			strings.HasPrefix(strings.ToUpper(name), "TF_") {
			continue
		}
		env = append(env, entry)
	}
	env = append(env, "TF_IN_AUTOMATION=1", "AWS_EC2_METADATA_DISABLED=true")
	run := func(wantSuccess bool, arguments ...string) string {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		command := exec.CommandContext(ctx, binary, arguments...)
		command.Dir = workspace
		command.Env = env
		command.Stdin = strings.NewReader("")
		output, err := command.CombinedOutput()
		if ctx.Err() != nil {
			t.Fatalf("quartz %v did not terminate with closed stdin: %v\n%s", arguments, ctx.Err(), output)
		}
		if (err == nil) != wantSuccess {
			t.Fatalf("quartz %v: success=%t, want=%t: %v\n%s", arguments, err == nil, wantSuccess, err, output)
		}
		return string(output)
	}
	run(true, "--version")
	run(true, "--help")
	schemaPath := filepath.Join(workspace, "quartz.schema.json")
	run(true, "--config", filepath.Join(workspace, "missing.yaml"), "schema", "--out", schemaPath)
	content, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatal(err)
	}
	var editorSchema struct {
		Version    string                     `json:"$schema"`
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(content, &editorSchema); err != nil {
		t.Fatal(err)
	}
	if editorSchema.Version != "https://json-schema.org/draft/2020-12/schema" ||
		editorSchema.Properties["name"] == nil || editorSchema.Properties["__internal__"] != nil {
		t.Fatal("native schema command did not produce the configuration editor contract")
	}
	if output := run(false, "--config", filepath.Join(workspace, "missing.yaml"), "schema", "--out", workspace); !strings.Contains(output, "write configuration schema") {
		t.Fatalf("schema write error was not reported explicitly:\n%s", output)
	}
	output := run(false, "--config", filepath.Join(workspace, "missing.yaml"), "render")
	if !strings.Contains(output, "load configuration") {
		t.Fatalf("missing configuration was not reported explicitly:\n%s", output)
	}
	run(true, "--config", config, "render", "--out", filepath.Join(workspace, "rendered.yaml"))
	run(true, "--config", config, "tofu", "init", "--stage", "local")
	run(true, "--config", config, "tofu", "apply", "--stage", "local")
	statePath := filepath.Join(stage, "terraform.tfstate")
	state := func() map[string]json.RawMessage {
		t.Helper()
		content, err := os.ReadFile(statePath)
		if err != nil {
			t.Fatal(err)
		}
		var result map[string]json.RawMessage
		if err := json.Unmarshal(content, &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	initial := state()
	if !strings.Contains(string(initial["outputs"]), "native-runtime-verified") {
		t.Fatalf("native apply did not persist the expected output: %s", initial["outputs"])
	}
	// A second independent process must reuse the cached executable and state.
	run(true, "--config", config, "tofu", "apply", "--stage", "local")
	repeated := state()
	if string(initial["serial"]) != string(repeated["serial"]) {
		t.Fatalf("idempotent apply changed the state serial: %s -> %s", initial["serial"], repeated["serial"])
	}
	run(true, "--config", config, "tofu", "destroy", "--stage", "local")
	destroyed := state()
	var resources []json.RawMessage
	if err := json.Unmarshal(destroyed["resources"], &resources); err != nil {
		t.Fatal(err)
	}
	if len(resources) != 0 {
		t.Fatalf("native destroy retained resources: %s", destroyed["resources"])
	}
	run(true, "--config", config, "tofu", "destroy", "--stage", "local")
}
