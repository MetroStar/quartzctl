package tofu

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MetroStar/quartzctl/internal/config"
	"github.com/MetroStar/quartzctl/internal/config/schema"
	"github.com/MetroStar/quartzctl/internal/privateaccess"
)

func TestPrivateKubernetesAndHelmProviderEnvironment(t *testing.T) {
	directory := ".private-provider-env-" + t.Name()
	os.MkdirAll(directory, 0700)
	defer os.RemoveAll(directory)
	t.Setenv(privateaccess.ProxyEnv, "http://127.0.0.1:12345")
	t.Setenv(privateaccess.ConfigEnv, filepath.Join(directory, "ephemeral.yaml"))
	t.Setenv("HTTP_PROXY", "http://original.example:3128")
	client := &TofuClient{cfg: config.Settings{Config: schema.QuartzConfig{Tmp: directory}}}
	stage := schema.StageConfig{}
	stage.Providers.Kubernetes = true
	environment := client.stageEnvironment(stage)
	if environment["KUBE_CONFIG_PATH"] != os.Getenv(privateaccess.ConfigEnv) ||
		environment["KUBECONFIG"] != os.Getenv(privateaccess.ConfigEnv) ||
		environment["KUBE_PROXY_URL"] != os.Getenv(privateaccess.ProxyEnv) {
		t.Fatal("Kubernetes and Helm providers must use the same ephemeral proxy kubeconfig")
	}
	if environment["HTTP_PROXY"] != "http://original.example:3128" {
		t.Fatal("Terraform setup must not replace a cached or unrelated global HTTP proxy")
	}
}
