//go:build !windows

package privateaccess

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func testNativePrivateProviders(t *testing.T, ctx context.Context, directory string, session *Session) {
	t.Helper()
	tofu, err := exec.LookPath("tofu")
	if err != nil {
		t.Log("OpenTofu missing: native private-provider integration not run")
		return
	}
	directory = filepath.Join(directory, "providers")
	if err := os.MkdirAll(filepath.Join(directory, "chart"), 0700); err != nil {
		t.Fatal(err)
	}
	configuration := `terraform {
  required_providers {
    kubernetes = { source = "hashicorp/kubernetes", version = "3.1.0" }
    helm = { source = "hashicorp/helm", version = "3.1.1" }
  }
}
provider "kubernetes" {}
provider "helm" {}
data "kubernetes_server_version" "current" {}
resource "helm_release" "fixture" {
  name = "fixture"
  namespace = "default"
  chart = "./chart"
}
`
	if err := os.WriteFile(filepath.Join(directory, "main.tf"), []byte(configuration), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "chart", "Chart.yaml"), []byte("apiVersion: v2\nname: fixture\nversion: 0.1.0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, arguments := range [][]string{{"init", "-backend=false", "-input=false"}, {"plan", "-input=false"}} {
		command := exec.CommandContext(ctx, tofu, arguments...)
		command.Dir = directory
		command.Env = append(os.Environ(),
			"KUBECONFIG="+session.KubeconfigPath, "KUBE_CONFIG_PATH="+session.KubeconfigPath,
			"KUBE_PROXY_URL="+session.ProxyURL, "TF_DATA_DIR="+filepath.Join(directory, "data"),
			"TF_CLI_CONFIG_FILE=/dev/null", "TF_CLI_ARGS_init=-plugin-dir=/home/ubuntu/.terraform.d/plugin-cache")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("actual Terraform Kubernetes/Helm providers did not use the private context: %v\n%s", err, output)
		}
	}
	t.Log("Actual Terraform Kubernetes/Helm providers planned against mocked private SSM TLS API.")
}
