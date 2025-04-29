package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/MetroStar/quartzctl/internal/config"
	"github.com/MetroStar/quartzctl/internal/provider"
	"github.com/MetroStar/quartzctl/internal/terraform"
	"github.com/MetroStar/quartzctl/internal/util"
	"github.com/knadh/koanf/v2"
	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestNewRootLoginCommand(t *testing.T) {
	p := defaultTestConfig(t)
	cmd := NewRootLoginCommand(p).Command

	assert.Equal(t, "login", cmd.Name)
	assert.Equal(t, "Generate a kubeconfig for the current cluster", cmd.Usage)
	assert.Len(t, cmd.Flags, 1)

	flag := cmd.Flags[0].(*cli.StringFlag)
	assert.Equal(t, "out", flag.Name)

	err := cmd.Action(context.Background(), &cli.Command{})
	assert.NoError(t, err)
}

func TestNewRootInfoCommand(t *testing.T) {
	p := defaultTestConfig(t)
	cmd := NewRootInfoCommand(p).Command

	assert.Equal(t, "info", cmd.Name)
	assert.Equal(t, "Output configuration info for the current cluster", cmd.Usage)

	err := cmd.Action(context.Background(), &cli.Command{})
	assert.NoError(t, err)
}

func TestNewRootCheckCommand(t *testing.T) {
	p := defaultTestConfig(t)
	cmd := NewRootCheckCommand(p).Command

	assert.Equal(t, "check", cmd.Name)
	assert.Equal(t, "Check environment and configuration for required values", cmd.Usage)

	err := cmd.Action(context.Background(), &cli.Command{})
	assert.NoError(t, err)
}

func TestNewRootRenderCommand(t *testing.T) {
	p := defaultTestConfig(t)
	cmd := NewRootRenderCommand(p).Command

	assert.Equal(t, "render", cmd.Name)
	assert.Equal(t, "Write fully rendered yaml config", cmd.Usage)
	assert.Len(t, cmd.Flags, 1)

	flag := cmd.Flags[0].(*cli.StringFlag)
	assert.Equal(t, "out", flag.Name)

	err := cmd.Run(context.Background(), []string{cmd.Name, "--out", filepath.Join(t.TempDir(), "render_test.yaml")})
	assert.NoError(t, err)
}

func TestNewRootRefreshSecretsCommand(t *testing.T) {
	p := defaultTestConfig(t)
	cmd := NewRootRefreshSecretsCommand(p).Command

	assert.Equal(t, "refresh-secrets", cmd.Name)
	assert.Equal(t, "Trigger all external secrets to be refreshed immediately", cmd.Usage)

	err := cmd.Action(context.Background(), &cli.Command{})
	assert.NoError(t, err)
}

func TestNewRootExportCommand(t *testing.T) {
	p := defaultTestConfig(t)
	cmd := NewRootExportCommand(p).Command

	assert.Equal(t, "export", cmd.Name)
	assert.Equal(t, "Export configured Kubernetes resources to yaml", cmd.Usage)

	err := cmd.Action(context.Background(), &cli.Command{})
	assert.NoError(t, err)
}

func TestNewRootRestartCommand(t *testing.T) {
	p := defaultTestConfig(t)
	cmd := NewRootRestartCommand(p).Command

	assert.Equal(t, "restart", cmd.Name)
	assert.Equal(t, "Restart target resource(s)", cmd.Usage)
	assert.Len(t, cmd.Flags, 3)

	err := cmd.Run(context.Background(), []string{cmd.Name, "--kind", "deployment"})
	assert.NoError(t, err)
}

func TestNewRootInternalCommand(t *testing.T) {
	p := defaultTestConfig(t)
	cmd := NewRootInternalCommand(p).Command

	assert.Equal(t, "internal", cmd.Name)
	assert.True(t, cmd.Hidden)
	assert.Len(t, cmd.Commands, 1)
	assert.Equal(t, "force-cleanup", cmd.Commands[0].Name)

	err := cmd.Commands[0].Action(context.Background(), &cli.Command{})
	assert.NoError(t, err)
}

func TestCmdVersion(t *testing.T) {
	// for coverage
	Version("v0.0.1-test", "")
	Version("v0.0.1-test", fmt.Sprintf("%d", time.Now().Unix()))
}

func TestVersion(t *testing.T) {
	var buf bytes.Buffer
	util.SetWriter(&buf)

	Version("1.0.0", "1672531200") // Unix timestamp for 2023-01-01
	output := buf.String()

	assert.Contains(t, output, "Quartz 1.0.0")
	assert.Contains(t, output, "Build Date: 2023-01-01")
}

func TestCmdRender(t *testing.T) {
	p := defaultTestConfig(t)

	tmp := t.TempDir()
	out := filepath.Join(tmp, "render_test.yaml")
	err := Render(context.Background(), out, p)
	if err != nil {
		t.Errorf("unexpected error in cmd Render, %v", err)
	}
}

func TestRender(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "output.yaml")

	mockSettings, _ := config.NewSettings(koanf.New("."), koanf.New("."))
	mockParams := &CommandParams{settings: &mockSettings}

	err := Render(context.Background(), outputPath, mockParams)
	assert.NoError(t, err)

	_, err = os.Stat(outputPath)
	assert.NoError(t, err, "Output file should exist")
}

func TestCmdClusterInfo(t *testing.T) {
	p := defaultTestConfig(t)

	// defaults to true, just specifying here to be explicit for the second case
	p.Settings().Config.Internal.Installer.Summary.Enabled = true

	err := ClusterInfo(context.Background(), p)
	if err != nil {
		t.Errorf("unexpected error in cmd ClusterInfo, %v", err)
	}

	// silently disables the summary output for dev environments where it's prone to failure
	p.Settings().Config.Internal.Installer.Summary.Enabled = false
	err = ClusterInfo(context.Background(), p)
	if err != nil {
		t.Errorf("unexpected error in cmd ClusterInfo, %v", err)
	}
}

func TestCmdClusterLogin(t *testing.T) {
	p := defaultTestConfig(t)

	tmp := t.TempDir()
	out := filepath.Join(tmp, "kubeconfig")
	err := ClusterLogin(context.Background(), out, p)
	if err != nil {
		t.Errorf("unexpected error in cmd ClusterLogin, %v", err)
	}
}

func TestCmdCheck(t *testing.T) {
	p := defaultTestConfig(t)
	Check(context.Background(), p)
}

func TestCmdRefreshSecrets(t *testing.T) {
	p := defaultTestConfig(t)

	err := RefreshSecrets(context.Background(), p)
	if err != nil {
		t.Errorf("unexpected error in cmd RefreshSecrets, %v", err)
	}
}

func TestCmdCleanup(t *testing.T) {
	p := defaultTestConfig(t)

	err := Cleanup(context.Background(), p)
	if err != nil {
		t.Errorf("unexpected error in cmd Cleanup, %v", err)
	}
}

func TestCmdBanner(t *testing.T) {
	Banner()
}

func TestCmdConfirm(t *testing.T) {
	p := defaultTestConfig(t)

	err := Confirm(context.Background(), "Are you sure you want to run this test?", p)
	if err != nil {
		t.Errorf("unexpected error in cmd Confirm, %v", err)
	}
}

func TestCmdExport(t *testing.T) {
	p := defaultTestConfig(t)

	p.Settings().Config.Export.Path = t.TempDir()

	err := Export(context.Background(), p)
	if err != nil {
		t.Errorf("unexpected error in cmd Export, %v", err)
	}
}

func TestCmdPrepareAccount(t *testing.T) {
	p := defaultTestConfig(t)

	err := PrepareAccount(context.Background(), p)
	if err != nil {
		t.Errorf("unexpected error in cmd PrepareAccount, %v", err)
	}
}

func defaultTestConfig(t *testing.T) *CommandParams {
	t.Setenv("SILENT", "1")

	terraform.ResetInstance()

	c := filepath.Join("testdata", "config.happy.yaml")
	s := filepath.Join("testdata", "secrets.happy.yaml")

	cfg, err := config.Load(context.Background(), c, s)
	if err != nil {
		t.Fatalf("unexpected error in default configure, %v", err)
	}

	cfg.Config.Tmp = t.TempDir()

	cm := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.Config.State.ConfigMapName,
			Namespace: cfg.Config.State.ConfigMapNamespace,
		},
		Data: map[string]string{
			"key1": "true",
		},
	}

	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testdeploy1",
			Namespace: "testns1",
		},
		Spec: appsv1.DeploymentSpec{},
	}
	udeployment, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(deployment)

	api := provider.NewKubernetesApiMock().WithDynamicObjects(
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "external-secrets.io/v1beta1",
				"kind":       "ExternalSecret",
				"metadata": map[string]interface{}{
					"namespace": "testns1",
					"name":      "testobj1",
				},
			},
		},
		// even though deployment is a standard type, needs to be added to the
		// dynamic client for the List() api to work
		&unstructured.Unstructured{
			Object: udeployment,
		},
	).WithClientObjects(
		cm,
		deployment,
	)

	kubeconfig := provider.KubeconfigInfo{}

	k8s, err := provider.NewKubernetesClient(api, kubeconfig, cfg.Config)
	if err != nil {
		t.Fatalf("unexpected error from kubernetes client constructor, %v", err)
	}

	p := CommandParams{
		settings: &cfg,
		provider: provider.NewProviderFactory(cfg.Config, cfg.Secrets, provider.WithKubernetesProvider(k8s)),
	}

	return &p
}
