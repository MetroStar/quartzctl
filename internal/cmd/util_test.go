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

package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/MetroStar/quartzctl/internal/config"
	"github.com/MetroStar/quartzctl/internal/config/schema"
	"github.com/MetroStar/quartzctl/internal/provider"
	"github.com/MetroStar/quartzctl/internal/stages"
	"github.com/MetroStar/quartzctl/internal/tofu"
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
	assert.Equal(t, "Generate/refresh a kubeconfig for the current cluster", cmd.Usage)
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
	assert.Len(t, cmd.Flags, 1)

	flag := cmd.Flags[0].(*cli.BoolFlag)
	assert.Equal(t, "ai-telemetry", flag.Name)

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
	assert.Len(t, cmd.Commands, 2)
	assert.Equal(t, "force-cleanup", cmd.Commands[0].Name)
	assert.Equal(t, "cleanup-terminating-pods", cmd.Commands[1].Name)

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

func TestCmdClusterInfoIncludesModelWarmerNote(t *testing.T) {
	p := defaultTestConfig(t)

	var buf bytes.Buffer
	util.SetWriter(&buf)
	t.Cleanup(func() { util.SetWriter(os.Stdout) })

	err := ClusterInfo(context.Background(), p)
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "Ollama model warming is still running in the background")
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

func TestIsClusterNotFoundError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"resource not found", errors.New("operation error EKS: DescribeCluster, ResourceNotFoundException: No cluster found for name: foo"), true},
		{"no cluster found", errors.New("No cluster found for name: foo"), true},
		{"status 404", errors.New("https response error StatusCode: 404, request id: abc"), true},
		{"transient connectivity", errors.New("dial tcp: i/o timeout"), false},
		{"unrelated", errors.New("some other error"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isClusterNotFoundError(tt.err); got != tt.want {
				t.Errorf("isClusterNotFoundError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestCmdCheck(t *testing.T) {
	p := defaultTestConfig(t)
	Check(context.Background(), p)
}

func TestLoadAITelemetryConfigFromConfigMap(t *testing.T) {
	api := provider.NewKubernetesApiMock().WithClientObjects(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "agentgateway",
			Name:      "agentgateway-telemetry-queries",
		},
		Data: map[string]string{
			"prometheusNamespace": "obs",
			"prometheusService":   "prom",
			"prometheusPort":      "9091",
			"window":              "30m",
			"query.scrape_up":     "vector(1)",
		},
	})
	k8s, err := provider.NewKubernetesClient(api, provider.KubeconfigInfo{}, schema.QuartzConfig{})
	assert.NoError(t, err)

	cfg := loadAITelemetryConfig(context.Background(), k8s)

	assert.Equal(t, "obs", cfg.PrometheusNamespace)
	assert.Equal(t, "prom", cfg.PrometheusService)
	assert.Equal(t, 9091, cfg.PrometheusPort)
	assert.Equal(t, "30m", cfg.Window)
	assert.Equal(t, "vector(1)", cfg.Queries["scrape_up"])
	assert.NotEmpty(t, cfg.Queries["recent_requests"])
}

func TestDefaultAITelemetryConfig(t *testing.T) {
	cfg := defaultAITelemetryConfig()

	assert.Equal(t, "monitoring", cfg.PrometheusNamespace)
	assert.Equal(t, "monitoring-monitoring-kube-prometheus", cfg.PrometheusService)
	assert.Equal(t, 9090, cfg.PrometheusPort)
	assert.Contains(t, cfg.Queries["recent_requests"], "agentgateway_requests_total")
	assert.Contains(t, cfg.Queries["recent_requests"], "agentgateway_gen_ai_server_request_duration_count")
}

func TestAITelemetryStatus(t *testing.T) {
	tests := []struct {
		key        string
		value      float64
		wantStatus string
	}{
		{key: "scrape_up", value: 1, wantStatus: "OK"},
		{key: "scrape_up", value: 0, wantStatus: "Missing"},
		{key: "recent_requests", value: 0, wantStatus: "No traffic"},
		{key: "recent_5xx", value: 0, wantStatus: "OK"},
		{key: "recent_5xx", value: 2, wantStatus: "Errors"},
		{key: "model_not_found", value: 0, wantStatus: "OK"},
		{key: "model_not_found", value: 1, wantStatus: "Errors"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s/%f", tt.key, tt.value), func(t *testing.T) {
			status, _ := aiTelemetryStatus(tt.key, tt.value)
			assert.Equal(t, tt.wantStatus, status)
		})
	}
}

func TestAITelemetryQueryTimeout(t *testing.T) {
	t.Setenv("QUARTZ_AI_TELEMETRY_TIMEOUT", "11s")
	assert.Equal(t, 11*time.Second, aiTelemetryQueryTimeout())

	t.Setenv("QUARTZ_AI_TELEMETRY_TIMEOUT", "bogus")
	assert.Equal(t, 8*time.Second, aiTelemetryQueryTimeout())
}

func TestSummarizeAITelemetryError(t *testing.T) {
	cfg := aiTelemetryConfig{
		PrometheusNamespace: "monitoring",
		PrometheusService:   "monitoring-monitoring-kube-prometheus",
		PrometheusPort:      9090,
	}

	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "deadline",
			err:  context.DeadlineExceeded,
			want: "timed out after 8s",
		},
		{
			name: "not found",
			err:  errors.New(`services "http:prometheus-operated:9090" not found`),
			want: "was not found",
		},
		{
			name: "service unavailable",
			err:  errors.New("the server is currently unable to handle the request"),
			want: "returned service unavailable",
		},
		{
			name: "generic",
			err:  errors.New("x509: certificate signed by unknown authority"),
			want: "failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := summarizeAITelemetryError(tt.err, cfg, 8*time.Second)
			assert.ErrorContains(t, err, "monitoring/monitoring-monitoring-kube-prometheus:9090")
			assert.ErrorContains(t, err, tt.want)
		})
	}
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

func TestCmdConfirmAssumeYes(t *testing.T) {
	p := defaultTestConfig(t)
	p.assumeYes = true

	err := Confirm(context.Background(), "Are you sure you want to run this test?", p)
	if err != nil {
		t.Errorf("unexpected error in cmd Confirm with assumeYes, %v", err)
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

func TestClassifySSO(t *testing.T) {
	tests := []struct {
		name           string
		app            string
		callbackPaths  []string
		public         bool
		wantSSO        string
		wantClientType string
		wantRealm      string
	}{
		{
			name:           "keycloak is the IdP itself",
			app:            "keycloak",
			callbackPaths:  nil,
			wantSSO:        "IdP (admin)",
			wantClientType: "—",
			wantRealm:      "master",
		},
		{
			name:           "no callbacks means no SSO",
			app:            "k8sgpt",
			callbackPaths:  nil,
			wantSSO:        "none",
			wantClientType: "—",
			wantRealm:      "—",
		},
		{
			name:           "oauth2-proxy callback path",
			app:            "epyon",
			callbackPaths:  []string{"/oauth2/callback"},
			wantSSO:        "OIDC (oauth2-proxy)",
			wantClientType: "confidential",
			wantRealm:      "infra",
		},
		{
			name:           "native confidential client",
			app:            "argocd",
			callbackPaths:  []string{"/auth/callback", "/api/dex/callback"},
			wantSSO:        "OIDC (native)",
			wantClientType: "confidential",
			wantRealm:      "infra",
		},
		{
			name:           "native public client",
			app:            "headlamp",
			callbackPaths:  []string{"/oidc-callback"},
			public:         true,
			wantSSO:        "OIDC (native)",
			wantClientType: "public",
			wantRealm:      "infra",
		},
		{
			name:           "sonarqube nested oauth2 path stays native",
			app:            "sonarqube",
			callbackPaths:  []string{"/oauth2/callback/oidc"},
			wantSSO:        "OIDC (native)",
			wantClientType: "confidential",
			wantRealm:      "infra",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sso, clientType, realm := classifySSO(tt.app, tt.callbackPaths, tt.public)
			assert.Equal(t, tt.wantSSO, sso)
			assert.Equal(t, tt.wantClientType, clientType)
			assert.Equal(t, tt.wantRealm, realm)
		})
	}
}

func TestTruncateDetail(t *testing.T) {
	tests := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{name: "short string unchanged", in: "ready", max: 10, want: "ready"},
		{name: "trims surrounding whitespace", in: "  ready  ", max: 10, want: "ready"},
		{name: "exact length unchanged", in: "abcde", max: 5, want: "abcde"},
		{name: "truncated with ellipsis", in: "abcdefghij", max: 5, want: "abcd…"},
		{name: "max of one returns single rune", in: "abcdef", max: 1, want: "a"},
		{name: "unicode counted by rune", in: "héllo wörld", max: 6, want: "héllo…"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, truncateDetail(tt.in, tt.max))
		})
	}
}

func TestBuildSSOSummaryRows(t *testing.T) {
	cfg := schema.QuartzConfig{
		Core: schema.InfrastructureEnvironmentConfig{
			Applications: map[string]schema.InfrastructureApplicationConfig{
				"argocd": {
					Description:  "ArgoCD",
					CallbackUrls: []schema.ApplicationCallbackConfig{{Path: "/auth/callback"}},
				},
				"epyon": {
					Description:  "Epyon",
					CallbackUrls: []schema.ApplicationCallbackConfig{{Path: "/oauth2/callback"}},
				},
				"keycloak": {
					Description: "Keycloak",
				},
				"disabled-app": {
					Description:  "Disabled",
					Disabled:     true,
					CallbackUrls: []schema.ApplicationCallbackConfig{{Path: "/cb"}},
				},
			},
		},
	}

	rows := buildSSOSummaryRows(cfg)

	// Build a lookup by package name for easy assertions.
	byPkg := make(map[string][]string, len(rows))
	for _, r := range rows {
		byPkg[r[0]] = r
	}

	// Configured apps with their derived posture.
	assert.Equal(t, []string{"ArgoCD", "OIDC (native)", "confidential", "infra"}, byPkg["ArgoCD"])
	assert.Equal(t, []string{"Epyon", "OIDC (oauth2-proxy)", "confidential", "infra"}, byPkg["Epyon"])
	assert.Equal(t, []string{"Keycloak", "IdP (admin)", "—", "master"}, byPkg["Keycloak"])

	// Backends without SSO are appended for a complete package picture.
	assert.Equal(t, []string{"agentgateway", "none", "—", "—"}, byPkg["agentgateway"])
	assert.Equal(t, []string{"k8sgpt", "none", "—", "—"}, byPkg["k8sgpt"])
	assert.Equal(t, []string{"kagent", "none", "—", "—"}, byPkg["kagent"])

	// Disabled apps are omitted.
	_, ok := byPkg["Disabled"]
	assert.False(t, ok)

	// Rows are sorted case-insensitively by package name.
	names := make([]string, 0, len(rows))
	for _, r := range rows {
		names = append(names, strings.ToLower(r[0]))
	}
	assert.True(t, slices.IsSorted(names))
}

func TestPrintSSOSummary(t *testing.T) {
	// Smoke test: the header is written via the util writer; the table body is
	// rendered to stdout by lipgloss. Verify the header is emitted and the call
	// does not panic for a representative config.
	var buf bytes.Buffer
	util.SetWriter(&buf)
	t.Cleanup(func() { util.SetWriter(os.Stdout) })

	cfg := schema.QuartzConfig{
		Core: schema.InfrastructureEnvironmentConfig{
			Applications: map[string]schema.InfrastructureApplicationConfig{
				"argocd": {
					Description:  "ArgoCD",
					CallbackUrls: []schema.ApplicationCallbackConfig{{Path: "/auth/callback"}},
				},
			},
		},
	}

	printSSOSummary(cfg)
	assert.Contains(t, buf.String(), "SSO summary")
}

func defaultTestConfig(t *testing.T) *CommandParams {
	t.Setenv("SILENT", "1")

	tofu.ResetInstance()

	c := filepath.Join("testdata", "config.happy.yaml")
	s := filepath.Join("testdata", "secrets.happy.yaml")

	cfg, err := config.Load(context.Background(), c, s)
	if err != nil {
		t.Fatalf("unexpected error in default configure, %v", err)
	}

	cfg.Config.Tmp = t.TempDir()
	// Redirect file-log artifacts (e.g. the clean teardown report written by
	// persistCleanupReport) into a per-test temp dir so Clean()-exercising tests
	// don't litter the repo working tree with a log/ directory.
	cfg.Config.Log.File.Path = filepath.Join(t.TempDir(), "$name.$date.log")
	cfg.Config.Log.Tofu.Path = filepath.Join(t.TempDir(), "$name.$date.tf.log")

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

	modelWarmerCM := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      modelWarmerStatusCM,
			Namespace: ollamaNamespace,
		},
		Data: map[string]string{
			"status.json": "\x1b[0m" + `{"phase":"Running","step":"pull","model":"gemma4:12b","detail":"downloading","desiredModels":"gemma4:e4b,gemma4:12b","pulledModels":"gemma4:e4b"}` + "\x00",
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
		modelWarmerCM,
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

func TestOnCheckStart(t *testing.T) {
	cr := stages.CheckResult{
		Id:    "test-check-id",
		Type:  "http",
		Stage: "bigbang",
		Event: "install",
	}

	// Should not panic
	onCheckStart(cr)
}

func TestOnCheckComplete(t *testing.T) {
	tests := []struct {
		name string
		cr   stages.CheckResult
	}{
		{
			name: "success case",
			cr: stages.CheckResult{
				Id:    "test-check-id",
				Type:  "http",
				Stage: "bigbang",
				Event: "install",
				Error: nil,
			},
		},
		{
			name: "error case",
			cr: stages.CheckResult{
				Id:    "test-check-id",
				Type:  "kubernetes",
				Stage: "bigbang",
				Event: "install",
				Error: fmt.Errorf("check failed: deployment not ready"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			onCheckComplete(tt.cr)
		})
	}
}

func TestOnCheckRetry(t *testing.T) {
	tests := []struct {
		name    string
		cr      stages.CheckResult
		attempt int
	}{
		{
			name: "first attempt with error",
			cr: stages.CheckResult{
				Id:    "test-check-id",
				Type:  "daemonset",
				Stage: "bigbang",
				Event: "install",
				Error: fmt.Errorf("not ready: 2/3 pods"),
			},
			attempt: 1,
		},
		{
			name: "intermediate attempt",
			cr: stages.CheckResult{
				Id:    "test-check-id",
				Type:  "daemonset",
				Stage: "bigbang",
				Event: "install",
				Error: fmt.Errorf("not ready: 2/3 pods"),
			},
			attempt: 3,
		},
		{
			name: "fifth attempt (detailed message)",
			cr: stages.CheckResult{
				Id:    "test-check-id",
				Type:  "daemonset",
				Stage: "bigbang",
				Event: "install",
				Error: fmt.Errorf("not ready: 2/3 pods"),
			},
			attempt: 5,
		},
		{
			name: "tenth attempt (detailed message)",
			cr: stages.CheckResult{
				Id:    "test-check-id",
				Type:  "daemonset",
				Stage: "bigbang",
				Event: "install",
				Error: nil,
			},
			attempt: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should not panic
			onCheckRetry(tt.cr, tt.attempt)
		})
	}
}
