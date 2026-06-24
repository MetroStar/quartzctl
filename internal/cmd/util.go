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
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/MetroStar/quartzctl/internal/config/schema"
	"github.com/MetroStar/quartzctl/internal/log"
	"github.com/MetroStar/quartzctl/internal/provider"
	"github.com/MetroStar/quartzctl/internal/stages"
	"github.com/MetroStar/quartzctl/internal/util"
	"github.com/urfave/cli/v3"
)

var (
	// checkOpts defines options for health checks, including callbacks for start, completion, and retries.
	checkOpts = &stages.CheckOpts{
		OnStart:    onCheckStart,
		OnComplete: onCheckComplete,
		OnRetry:    onCheckRetry,
	}
)

func NewRootLoginCommand(p *CommandParams) RootCommandResult {
	return RootCommandResult{
		Command: &cli.Command{
			Name:    "login",
			Aliases: []string{"kubeconfig", "refresh-kubeconfig"},
			Usage:   "Generate/refresh a kubeconfig for the current cluster",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "out", Aliases: []string{"o"}, Usage: "output path (defaults to the configured kubeconfig path when empty)", Value: "./out/kubeconfig"},
			},
			Action: func(ctx context.Context, ccmd *cli.Command) error {
				path := ccmd.String("out")
				return ClusterLogin(ctx, path, p)
			},
		},
	}
}

func NewRootInfoCommand(p *CommandParams) RootCommandResult {
	return RootCommandResult{
		Command: &cli.Command{
			Name:  "info",
			Usage: "Output configuration info for the current cluster",
			Action: func(ctx context.Context, ccmd *cli.Command) error {
				return ClusterInfo(ctx, p)
			},
		},
	}
}

func NewRootCheckCommand(p *CommandParams) RootCommandResult {
	return RootCommandResult{
		Command: &cli.Command{
			Name:  "check",
			Usage: "Check environment and configuration for required values",
			Flags: []cli.Flag{
				&cli.BoolFlag{Name: "ai-telemetry", Usage: "Check Agent Gateway AI telemetry in Prometheus"},
				&cli.BoolFlag{Name: "install-readiness", Usage: "Check lightweight post-install readiness signals for Flux, app delivery, and the AI stack"},
			},
			Action: func(ctx context.Context, ccmd *cli.Command) error {
				if ccmd.Bool("ai-telemetry") {
					return CheckAITelemetry(ctx, p)
				}
				if ccmd.Bool("install-readiness") {
					return CheckInstallReadiness(ctx, p)
				}
				Check(ctx, p)
				return nil
			},
		},
	}
}

func NewRootRenderCommand(p *CommandParams) RootCommandResult {
	return RootCommandResult{
		Command: &cli.Command{
			Name:  "render",
			Usage: "Write fully rendered yaml config",
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "out", Aliases: []string{"o"}, Usage: "output path", Value: "./out/quartz.generated.yaml"},
			},
			Action: func(ctx context.Context, ccmd *cli.Command) error {
				path := ccmd.String("out")
				return Render(ctx, path, p)
			},
		},
	}
}

func NewRootRefreshSecretsCommand(p *CommandParams) RootCommandResult {
	return RootCommandResult{
		Command: &cli.Command{
			Name:    "refresh-secrets",
			Aliases: []string{"rs"},
			Usage:   "Trigger all external secrets to be refreshed immediately",
			Action: func(ctx context.Context, ccmd *cli.Command) error {
				return RefreshSecrets(ctx, p)
			},
		},
	}
}

func NewRootExportCommand(p *CommandParams) RootCommandResult {
	return RootCommandResult{
		Command: &cli.Command{
			Name:  "export",
			Usage: "Export configured Kubernetes resources to yaml",
			Action: func(ctx context.Context, ccmd *cli.Command) error {
				return Export(ctx, p)
			},
		},
	}
}

func NewRootRestartCommand(p *CommandParams) RootCommandResult {
	return RootCommandResult{
		Command: &cli.Command{
			Name:  "restart",
			Usage: "Restart target resource(s)",
			Flags: []cli.Flag{
				&cli.StringSliceFlag{Name: "kind", Aliases: []string{"k"}, Usage: "Resource kind", Required: false},
				&cli.StringFlag{Name: "namespace", Aliases: []string{"n"}, Usage: "Namespace", Required: false},
				&cli.StringFlag{Name: "name", Usage: "Name", Required: false},
			},
			Action: func(ctx context.Context, ccmd *cli.Command) error {
				kinds := ccmd.StringSlice("kind")
				ns := ccmd.String("namespace")
				name := ccmd.String("name")

				if len(kinds) == 0 {
					kinds = []string{"deployment", "daemonset", "statefulset"}
				}

				for _, k := range kinds {
					err := Restart(ctx, k, ns, name, p)
					if err != nil {
						return err
					}
				}

				return nil
			},
		},
	}
}

// Version displays the version of the Quartz installer along with the build date.
//
// Parameters:
//   - version: The version of the Quartz installer.
//   - buildDate: The build date of the Quartz installer.
func Version(version string, buildDate string) {
	log.Debug("Entering", "command", "version")
	defer log.Debug("Completed", "command", "version")

	var format = "2006-01-02 15:04 MST"

	d := buildDate
	if d == "" {
		d = time.Now().UTC().Format(format)
	} else {
		d, _, _ = strings.Cut(d, ".") // in case a float was passed in
		c, err := strconv.ParseInt(d, 10, 64)
		if err != nil {
			panic(err)
		}
		d = time.Unix(c, 0).Format(format)
	}
	util.Msgf("Quartz %s\nBuild Date: %s\n", version, d)
}

// Render writes the full configuration to the specified file path.
//
// Parameters:
//   - ctx: The context for the operation.
//   - path: The file path where the configuration will be written.
//   - p: *CommandParams containing configuration and runtime parameters.
//
// Returns:
//   - error: An error if the rendering fails, otherwise nil.
func Render(ctx context.Context, path string, p *CommandParams) error {
	log.Debug("Entering", "command", "render")
	defer log.Debug("Completed", "command", "render")

	f, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	util.Msgf("Writing generated Quartz YAML to %s", f)
	return p.Settings().WriteYamlConfig(f)
}

// ClusterInfo retrieves and displays information about the Quartz cluster.
//
// Parameters:
//   - ctx: The context for the operation.
//   - p: *CommandParams containing configuration and runtime parameters.
//
// Returns:
//   - error: An error if retrieving cluster information fails, otherwise nil.
func ClusterInfo(ctx context.Context, p *CommandParams) error {
	log.Debug("Entering", "command", "clusterInfo")
	defer log.Debug("Completed", "command", "clusterInfo")

	if !p.Settings().Config.Internal.Installer.Summary.Enabled {
		log.Warn("Summary disabled in config")
		return nil
	}

	util.Hdr("Cluster summary")
	cp, _ := p.Provider().Cloud(ctx)
	err := cp.PrintClusterInfo(ctx)
	if err != nil {
		return err
	}

	k8s, err := p.Provider().Kubernetes(ctx)
	if err != nil {
		return err
	}

	// Flux HelmRelease reconciliation status and the SSO posture per package,
	// shown before the per-application connection details.
	printHelmReleaseStatus(ctx, k8s)
	printSSOSummary(p.Settings().Config)

	k8s.PrintClusterInfo(ctx)

	util.Msgf("export KUBECONFIG=%s", p.Settings().Config.KubeconfigPath())
	printBackgroundTasks(ctx, p)

	return err
}

func printBackgroundTasks(ctx context.Context, p *CommandParams) {
	fmt.Println()
	util.Printf("Background tasks")
	util.Msg("CI/CD initial builds and first promotion checks may take up to 15 minutes after install; track progress in Jenkins and ArgoCD.")
	ReportAppDeliveryReadiness(ctx, p)
	ReportModelWarmerStatus(ctx, p)
}

// printHelmReleaseStatus renders a table of every Flux HelmRelease and its
// current reconciliation state. It reuses the read-only ClusterProgressSnapshot
// already collected for install convergence, so it adds no extra cluster load
// and degrades gracefully when the Flux CRDs are absent (no table is printed).
func printHelmReleaseStatus(ctx context.Context, k8s provider.KubernetesProviderClient) {
	progress, err := k8s.ClusterProgressSnapshot(ctx)
	if err != nil {
		log.Debug("Skipping HelmRelease status table", "err", err)
		return
	}
	if len(progress.Releases) == 0 {
		return
	}

	releases := append([]provider.HelmReleaseStatus(nil), progress.Releases...)
	slices.SortFunc(releases, func(a, b provider.HelmReleaseStatus) int {
		return cmp.Compare(a.ID(), b.ID())
	})

	rows := make([][]string, 0, len(releases))
	for _, r := range releases {
		status := "Ready"
		detail := r.ReadyMsg
		switch {
		case r.Ready:
			// keep defaults
		case r.Stalled:
			status = "Stalled"
			detail = r.StalledMsg
		default:
			status = "Not Ready"
		}
		rows = append(rows, []string{r.Namespace, r.Name, status, truncateDetail(detail, 60)})
	}

	fmt.Println()
	util.Printf("Flux HelmReleases (%d/%d ready)", progress.HelmReleasesReady, progress.HelmReleasesTotal)
	util.PrintRowStatusTable(
		[]string{"Namespace", "Name", "Status", "Detail"},
		rows,
		func(_ int, row []string) util.RowStatus {
			switch row[2] {
			case "Ready":
				return util.StatusOk
			case "Stalled":
				return util.StatusError
			default:
				return util.StatusWarning
			}
		},
	)
}

// ssoBackendsWithoutSSO lists Quartz packages deployed without any Keycloak
// client (no SSO integration). They have no entry in the infra application
// config, so they are appended explicitly to keep the SSO summary an accurate
// picture of the full package set surfaced by `quartz info`.
var ssoBackendsWithoutSSO = []string{"agentgateway", "k8sgpt", "kagent"}

// printSSOSummary renders the SSO posture of each Quartz package: the
// authentication method, the Keycloak client type, and the realm it
// authenticates against. All application SSO is brokered through the single
// Keycloak "infra" realm; the "master" realm is the IdP admin realm used only
// by Keycloak itself.
func printSSOSummary(cfg schema.QuartzConfig) {
	rows := buildSSOSummaryRows(cfg)
	if len(rows) == 0 {
		return
	}

	fmt.Println()
	util.Printf("SSO summary")
	util.PrintTable([]string{"Package", "SSO", "Client Type", "Realm"}, rows)
}

// buildSSOSummaryRows derives the SSO summary table rows from the infra
// application config. Disabled applications are omitted, and packages deployed
// without any Keycloak client are appended so the summary reflects the full set
// of packages surfaced by `quartz info`. Rows are sorted by package name.
func buildSSOSummaryRows(cfg schema.QuartzConfig) [][]string {
	configured := make(map[string]bool, len(cfg.Core.Applications))
	var rows [][]string

	for name, app := range cfg.Core.Applications {
		configured[name] = true
		if app.Disabled {
			continue
		}

		pkg := app.Description
		if pkg == "" {
			pkg = name
		}

		paths := make([]string, 0, len(app.CallbackUrls))
		for _, cb := range app.CallbackUrls {
			paths = append(paths, cb.Path)
		}

		sso, clientType, realm := classifySSO(name, paths, app.Public)
		rows = append(rows, []string{pkg, sso, clientType, realm})
	}

	for _, name := range ssoBackendsWithoutSSO {
		if configured[name] {
			continue
		}
		rows = append(rows, []string{name, "none", "—", "—"})
	}

	slices.SortFunc(rows, func(a, b []string) int {
		return cmp.Compare(strings.ToLower(a[0]), strings.ToLower(b[0]))
	})

	return rows
}

// classifySSO derives the SSO posture of an infra application from its callback
// configuration. It returns the authentication method, Keycloak client type,
// and the realm the package authenticates against.
func classifySSO(name string, callbackPaths []string, public bool) (sso string, clientType string, realm string) {
	// Keycloak is the identity provider itself: it brokers SSO for everything
	// else and its own admin console authenticates against the master realm.
	if name == "keycloak" {
		return "IdP (admin)", "—", "master"
	}

	// No registered callback means no Keycloak client, i.e. no SSO.
	if len(callbackPaths) == 0 {
		return "none", "—", "—"
	}

	// Packages without native OIDC support are fronted by an oauth2-proxy
	// sidecar that performs the Keycloak login (callback path /oauth2/callback).
	for _, p := range callbackPaths {
		if p == "/oauth2/callback" {
			return "OIDC (oauth2-proxy)", "confidential", "infra"
		}
	}

	if public {
		return "OIDC (native)", "public", "infra"
	}
	return "OIDC (native)", "confidential", "infra"
}

// truncateDetail trims surrounding whitespace and caps a message at max runes,
// appending an ellipsis when it overflows, so status detail columns stay within
// a readable width.
func truncateDetail(s string, max int) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 1 {
		return string(r[:max])
	}
	return string(r[:max-1]) + "…"
}

// ClusterLogin generates a kubeconfig file for the Quartz environment.
//
// Parameters:
//   - ctx: The context for the operation.
//   - path: The file path where the kubeconfig will be written.
//   - p: *CommandParams containing configuration and runtime parameters.
//
// Returns:
//   - error: An error if generating the kubeconfig fails, otherwise nil.
func ClusterLogin(ctx context.Context, path string, p *CommandParams) error {
	log.Debug("Entering", "command", "clusterLogin")
	defer log.Debug("Completed", "command", "clusterLogin")

	util.Hdr("Generate kubeconfig")

	k8sClient, err := p.Provider().Kubernetes(ctx)
	if err != nil {
		// If the cluster no longer exists (e.g. a re-run of clean after the
		// EKS cluster has already been destroyed), there is nothing to log
		// into and no in-cluster (kubernetes/helm) resources left to manage.
		// Treat this like the kubeconfig-write failure below and continue,
		// so a service-dependent stage's destroy isn't blocked by a 404 from
		// DescribeCluster and the state backend teardown can proceed.
		if isClusterNotFoundError(err) {
			util.Msgf("Cluster not found, skipping kubeconfig generation: %v", err)
			return nil
		}
		return err
	}

	if path == "" {
		path = p.Settings().Config.KubeconfigPath()
	}

	err = k8sClient.WriteKubeconfigFile(path)
	if err != nil {
		util.Msgf("Failed to write kubeconfig %v", err)
		return nil // suppressing error here, if the cluster is unavailable it's not always a blocker downstream
	}

	util.Msgf("Kubeconfig written to %s", path)
	return nil
}

// isClusterNotFoundError reports whether err indicates the target EKS cluster
// no longer exists. This distinguishes a permanently-absent cluster (safe to
// ignore when generating a kubeconfig) from a transient connectivity failure
// (which should still surface). The EKS DescribeCluster API returns a 404 with
// a ResourceNotFoundException ("No cluster found for name: ...") when the
// cluster has been deleted.
func isClusterNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	notFoundPatterns := []string{
		"ResourceNotFoundException",
		"No cluster found",
		"StatusCode: 404",
	}
	for _, pattern := range notFoundPatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}
	return false
}

// Check verifies the dependencies required for Quartz installation.
//
// Parameters:
//   - ctx: The context for the operation.
//   - p: *CommandParams containing configuration and runtime parameters.
func Check(ctx context.Context, p *CommandParams) {
	log.Debug("Entering", "command", "check")
	defer log.Debug("Completed", "command", "check")

	util.Hdr("Check")

	opts := provider.NewProviderCheckOpts(ctx, *p.Provider())
	provider.Check(ctx, &opts)
}

type aiTelemetryConfig struct {
	PrometheusNamespace string
	PrometheusService   string
	PrometheusPort      int
	Window              string
	Queries             map[string]string
}

type aiTelemetryMetric struct {
	Key    string
	Name   string
	Value  float64
	Error  error
	Status string
	Detail string
}

var aiTelemetryQueryKeys = []string{
	"scrape_up",
	"recent_requests",
	"recent_2xx",
	"recent_5xx",
	"model_not_found",
}

func aiTelemetryQueryTimeout() time.Duration {
	const def = 8 * time.Second
	if raw := strings.TrimSpace(os.Getenv("QUARTZ_AI_TELEMETRY_TIMEOUT")); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil && d > 0 {
			return d
		}
	}
	return def
}

func defaultAITelemetryConfig() aiTelemetryConfig {
	return aiTelemetryConfig{
		PrometheusNamespace: "monitoring",
		PrometheusService:   "monitoring-monitoring-kube-prometheus",
		PrometheusPort:      9090,
		Window:              "15m",
		Queries: map[string]string{
			"scrape_up":       `sum(up{namespace="agentgateway", job=~"quartz-agentgateway.*|agentgateway.*"}) or vector(0)`,
			"recent_requests": `(sum(increase(agentgateway_requests_total{namespace="agentgateway"}[15m])) or sum(increase(agentgateway_request_duration_seconds_count{namespace="agentgateway"}[15m])) or sum(increase(agentgateway_gen_ai_server_request_duration_count{namespace="agentgateway"}[15m])) or vector(0))`,
			"recent_2xx":      `(sum(increase(agentgateway_requests_total{namespace="agentgateway",status=~"2.."}[15m])) or sum(increase(agentgateway_request_duration_seconds_count{namespace="agentgateway",status=~"2.."}[15m])) or sum(increase(agentgateway_requests_total{namespace="agentgateway",code=~"2.."}[15m])) or sum(increase(agentgateway_request_duration_seconds_count{namespace="agentgateway",code=~"2.."}[15m])) or sum(increase(agentgateway_requests_total{namespace="agentgateway",status_code=~"2.."}[15m])) or sum(increase(agentgateway_request_duration_seconds_count{namespace="agentgateway",status_code=~"2.."}[15m])) or sum(increase(agentgateway_requests_total{namespace="agentgateway",response_code=~"2.."}[15m])) or sum(increase(agentgateway_request_duration_seconds_count{namespace="agentgateway",response_code=~"2.."}[15m])) or vector(0))`,
			"recent_5xx":      `(sum(increase(agentgateway_requests_total{namespace="agentgateway",status=~"5.."}[15m])) or sum(increase(agentgateway_request_duration_seconds_count{namespace="agentgateway",status=~"5.."}[15m])) or sum(increase(agentgateway_requests_total{namespace="agentgateway",code=~"5.."}[15m])) or sum(increase(agentgateway_request_duration_seconds_count{namespace="agentgateway",code=~"5.."}[15m])) or sum(increase(agentgateway_requests_total{namespace="agentgateway",status_code=~"5.."}[15m])) or sum(increase(agentgateway_request_duration_seconds_count{namespace="agentgateway",status_code=~"5.."}[15m])) or sum(increase(agentgateway_requests_total{namespace="agentgateway",response_code=~"5.."}[15m])) or sum(increase(agentgateway_request_duration_seconds_count{namespace="agentgateway",response_code=~"5.."}[15m])) or vector(0))`,
			"model_not_found": `(sum(increase(agentgateway_requests_total{namespace="agentgateway",status="404"}[15m])) or sum(increase(agentgateway_request_duration_seconds_count{namespace="agentgateway",status="404"}[15m])) or sum(increase(agentgateway_requests_total{namespace="agentgateway",code="404"}[15m])) or sum(increase(agentgateway_request_duration_seconds_count{namespace="agentgateway",code="404"}[15m])) or sum(increase(agentgateway_requests_total{namespace="agentgateway",status_code="404"}[15m])) or sum(increase(agentgateway_request_duration_seconds_count{namespace="agentgateway",status_code="404"}[15m])) or sum(increase(agentgateway_requests_total{namespace="agentgateway",response_code="404"}[15m])) or sum(increase(agentgateway_request_duration_seconds_count{namespace="agentgateway",response_code="404"}[15m])) or sum(increase(agentgateway_requests_total{namespace="agentgateway",error=~".*(model|not.?found|not_found).*"}[15m])) or sum(increase(agentgateway_request_duration_seconds_count{namespace="agentgateway",error=~".*(model|not.?found|not_found).*"}[15m])) or sum(increase(agentgateway_requests_total{namespace="agentgateway",error_type=~".*(model|not.?found|not_found).*"}[15m])) or sum(increase(agentgateway_request_duration_seconds_count{namespace="agentgateway",error_type=~".*(model|not.?found|not_found).*"}[15m])) or vector(0))`,
		},
	}
}

func loadAITelemetryConfig(ctx context.Context, k8s provider.KubernetesProviderClient) aiTelemetryConfig {
	cfg := defaultAITelemetryConfig()

	data, err := k8s.GetConfigMapValue(ctx, "agentgateway", "agentgateway-telemetry-queries")
	if err != nil {
		log.Debug("Using default Agent Gateway telemetry queries", "err", err)
		return cfg
	}

	if v := strings.TrimSpace(data["prometheusNamespace"]); v != "" {
		cfg.PrometheusNamespace = v
	}
	if v := strings.TrimSpace(data["prometheusService"]); v != "" {
		cfg.PrometheusService = v
	}
	if v := strings.TrimSpace(data["prometheusPort"]); v != "" {
		if port, err := strconv.Atoi(v); err == nil && port > 0 {
			cfg.PrometheusPort = port
		}
	}
	if v := strings.TrimSpace(data["window"]); v != "" {
		cfg.Window = v
	}
	for _, key := range aiTelemetryQueryKeys {
		if q := strings.TrimSpace(data["query."+key]); q != "" {
			cfg.Queries[key] = q
		}
	}

	return cfg
}

func CheckAITelemetry(ctx context.Context, p *CommandParams) error {
	log.Debug("Entering", "command", "checkAITelemetry")
	defer log.Debug("Completed", "command", "checkAITelemetry")

	k8s, err := p.Provider().Kubernetes(ctx)
	if err != nil {
		return err
	}

	cfg := loadAITelemetryConfig(ctx, k8s)

	util.Hdr("Agent Gateway AI telemetry")
	util.Msgf("Prometheus: %s/%s:%d", cfg.PrometheusNamespace, cfg.PrometheusService, cfg.PrometheusPort)

	metrics := []aiTelemetryMetric{
		queryAITelemetryMetric(ctx, k8s, cfg, "scrape_up", "Scrape targets up"),
		queryAITelemetryMetric(ctx, k8s, cfg, "recent_requests", fmt.Sprintf("Requests (%s)", cfg.Window)),
		queryAITelemetryMetric(ctx, k8s, cfg, "recent_2xx", fmt.Sprintf("2xx responses (%s)", cfg.Window)),
		queryAITelemetryMetric(ctx, k8s, cfg, "recent_5xx", fmt.Sprintf("5xx responses (%s)", cfg.Window)),
		queryAITelemetryMetric(ctx, k8s, cfg, "model_not_found", fmt.Sprintf("Model-not-found/404 (%s)", cfg.Window)),
	}

	rows := make([][]string, 0, len(metrics))
	queryFailures := 0
	for _, m := range metrics {
		if m.Error != nil {
			queryFailures++
			rows = append(rows, []string{m.Name, "-", "Query failed", truncateDetail(m.Error.Error(), 80)})
			continue
		}

		rows = append(rows, []string{m.Name, formatTelemetryValue(m.Value), m.Status, m.Detail})
	}

	util.PrintRowStatusTable(
		[]string{"Metric", "Value", "Status", "Detail"},
		rows,
		func(_ int, row []string) util.RowStatus {
			switch row[2] {
			case "OK":
				return util.StatusOk
			case "No traffic":
				return util.StatusWarning
			default:
				return util.StatusError
			}
		},
	)

	if queryFailures == len(metrics) {
		return fmt.Errorf("all Agent Gateway telemetry queries failed")
	}

	return nil
}

func CheckInstallReadiness(ctx context.Context, p *CommandParams) error {
	log.Debug("Entering", "command", "checkInstallReadiness")
	defer log.Debug("Completed", "command", "checkInstallReadiness")

	k8s, err := p.Provider().Kubernetes(ctx)
	if err != nil {
		return err
	}

	util.Hdr("Install readiness")

	progress, progressErr := k8s.ClusterProgressSnapshot(ctx)
	appDelivery, appDeliveryOK, appDeliveryErr := readAppDeliveryReadiness(ctx, k8s)
	modelWarmer, modelWarmerOK, modelWarmerErr := readModelWarmerStatus(ctx, k8s)

	rows := [][]string{}

	if progressErr != nil {
		rows = append(rows, []string{"Flux HelmReleases", "-", "Unavailable", truncateDetail(progressErr.Error(), 80)})
	} else {
		status := "OK"
		detail := progress.SummaryWithStragglers(3)
		value := fmt.Sprintf("%d/%d ready", progress.HelmReleasesReady, progress.HelmReleasesTotal)
		if progress.HelmReleasesReady != progress.HelmReleasesTotal {
			status = "Waiting"
		}
		rows = append(rows, []string{"Flux HelmReleases", value, status, truncateDetail(detail, 80)})
	}

	switch {
	case appDeliveryErr != nil:
		rows = append(rows, []string{"App delivery bootstrap", "-", "Unavailable", truncateDetail(appDeliveryErr.Error(), 80)})
	case !appDeliveryOK:
		rows = append(rows, []string{"App delivery bootstrap", "-", "Pending", "Readiness ConfigMap not published yet"})
	default:
		status := "OK"
		value := fmt.Sprintf("%d/%d apps", appDelivery.ObservedApplications, appDelivery.ExpectedApplications)
		detail := appDelivery.Summary
		if !strings.EqualFold(appDelivery.Phase, "ready") {
			status = "Waiting"
		}
		rows = append(rows, []string{"App delivery bootstrap", value, status, truncateDetail(detail, 80)})
	}

	switch {
	case modelWarmerErr != nil:
		rows = append(rows, []string{"Ollama model warming", "-", "Unavailable", truncateDetail(modelWarmerErr.Error(), 80)})
	case !modelWarmerOK:
		rows = append(rows, []string{"Ollama model warming", "-", "Pending", "Model warmer status not published yet"})
	default:
		status := "OK"
		if strings.EqualFold(modelWarmer.Phase, "failed") {
			status = "Errors"
		} else if !strings.EqualFold(modelWarmer.Phase, "succeeded") && !modelWarmer.PersistentReady {
			status = "Waiting"
		}
		rows = append(rows, []string{
			"Ollama model warming",
			modelWarmerReadinessSummary(modelWarmer),
			status,
			truncateDetail(modelWarmerProgress(modelWarmer), 80),
		})
	}

	util.PrintRowStatusTable(
		[]string{"Signal", "Value", "Status", "Detail"},
		rows,
		func(_ int, row []string) util.RowStatus {
			switch row[2] {
			case "OK":
				return util.StatusOk
			case "Waiting":
				return util.StatusWarning
			default:
				return util.StatusError
			}
		},
	)

	if progressErr == nil && progress.HelmReleasesReady != progress.HelmReleasesTotal {
		return fmt.Errorf("not all HelmReleases are ready")
	}
	if appDeliveryErr == nil && appDeliveryOK && !strings.EqualFold(appDelivery.Phase, "ready") {
		return fmt.Errorf("app delivery bootstrap is still pending")
	}
	if modelWarmerErr == nil && modelWarmerOK && strings.EqualFold(modelWarmer.Phase, "failed") {
		return fmt.Errorf("ollama model warmer failed")
	}
	return nil
}

func queryAITelemetryMetric(ctx context.Context, k8s provider.KubernetesProviderClient, cfg aiTelemetryConfig, key string, name string) aiTelemetryMetric {
	query := cfg.Queries[key]
	if strings.TrimSpace(query) == "" {
		return aiTelemetryMetric{Key: key, Name: name, Error: fmt.Errorf("missing Prometheus query %q", key)}
	}

	timeout := aiTelemetryQueryTimeout()
	queryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	res, err := k8s.QueryPrometheus(queryCtx, cfg.PrometheusNamespace, cfg.PrometheusService, cfg.PrometheusPort, query)
	if err != nil {
		return aiTelemetryMetric{
			Key:   key,
			Name:  name,
			Error: summarizeAITelemetryError(err, cfg, timeout),
		}
	}

	value, ok := res.ScalarValue()
	if !ok {
		return aiTelemetryMetric{Key: key, Name: name, Error: fmt.Errorf("Prometheus query %q returned no scalar value", key)}
	}

	status, detail := aiTelemetryStatus(key, value)
	return aiTelemetryMetric{
		Key:    key,
		Name:   name,
		Value:  value,
		Status: status,
		Detail: detail,
	}
}

func summarizeAITelemetryError(err error, cfg aiTelemetryConfig, timeout time.Duration) error {
	if err == nil {
		return nil
	}

	target := fmt.Sprintf("%s/%s:%d", cfg.PrometheusNamespace, cfg.PrometheusService, cfg.PrometheusPort)
	msg := err.Error()
	lower := strings.ToLower(msg)

	switch {
	case errors.Is(err, context.DeadlineExceeded) || strings.Contains(lower, "context deadline exceeded"):
		return fmt.Errorf("Prometheus query target %s timed out after %s; Prometheus may be unavailable or both the port-forward and Kubernetes service-proxy paths are under pressure", target, timeout)
	case strings.Contains(lower, "not found") && strings.Contains(lower, "services"):
		return fmt.Errorf("Prometheus query target %s was not found; verify monitoring is installed and the telemetry ConfigMap points at the right service", target)
	case strings.Contains(lower, "service unavailable") || strings.Contains(lower, "currently unable to handle the request") || strings.Contains(lower, "503"):
		return fmt.Errorf("Prometheus query target %s returned service unavailable; monitoring may still be starting or the API service proxy may be overloaded", target)
	default:
		return fmt.Errorf("Prometheus query target %s failed: %w", target, err)
	}
}

func aiTelemetryStatus(key string, value float64) (string, string) {
	switch key {
	case "scrape_up":
		if value > 0 {
			return "OK", "Prometheus is scraping Agent Gateway metrics"
		}
		return "Missing", "No live Agent Gateway scrape target is up"
	case "recent_requests":
		if value > 0 {
			return "OK", "Recent AI traffic reached Agent Gateway"
		}
		return "No traffic", "Generate Open-WebUI, Kagent, K8sGPT, Epyon, or Jenkins traffic and rerun"
	case "recent_2xx":
		if value > 0 {
			return "OK", "Recent successful responses were observed"
		}
		return "No traffic", "No recent successful responses were observed"
	case "recent_5xx":
		if value == 0 {
			return "OK", "No recent server-side Agent Gateway failures"
		}
		return "Errors", "Recent 5xx responses were observed"
	case "model_not_found":
		if value == 0 {
			return "OK", "No recent 404/model-not-found responses"
		}
		return "Errors", "Recent 404/model-not-found responses were observed"
	default:
		return "OK", ""
	}
}

func formatTelemetryValue(v float64) string {
	s := fmt.Sprintf("%.2f", v)
	s = strings.TrimRight(s, "0")
	return strings.TrimRight(s, ".")
}

// RefreshSecrets triggers an immediate refresh of all external secrets.
//
// Parameters:
//   - ctx: The context for the operation.
//   - p: *CommandParams containing configuration and runtime parameters.
//
// Returns:
//   - error: An error if refreshing secrets fails, otherwise nil.
func RefreshSecrets(ctx context.Context, p *CommandParams) error {
	log.Debug("Entering", "command", "refreshSecrets")
	defer log.Debug("Completed", "command", "refreshSecrets")

	k8s, err := p.Provider().Kubernetes(ctx)
	if err != nil {
		return err
	}

	refreshed, err := k8s.RefreshExternalSecrets(ctx)
	if err != nil {
		util.Errorf("Failed to refresh secrets, %v", err)
		return err
	}
	if len(refreshed) == 0 {
		util.Msg("No ExternalSecrets were found to refresh")
		return nil
	}
	util.Msgf("Triggered refresh of %d ExternalSecret(s) across %d namespace(s): %s",
		len(refreshed), countResourceNamespaces(refreshed), summarizeResourceNamespaces(refreshed, 6))

	return nil
}

func countResourceNamespaces(resources []provider.KubernetesResource) int {
	seen := map[string]bool{}
	for _, resource := range resources {
		if resource.Namespace != "" {
			seen[resource.Namespace] = true
		}
	}
	return len(seen)
}

func summarizeResourceNamespaces(resources []provider.KubernetesResource, limit int) string {
	counts := map[string]int{}
	for _, resource := range resources {
		if resource.Namespace == "" {
			continue
		}
		counts[resource.Namespace]++
	}
	if len(counts) == 0 {
		return "none"
	}

	namespaces := make([]string, 0, len(counts))
	for ns := range counts {
		namespaces = append(namespaces, ns)
	}
	slices.Sort(namespaces)
	if limit > 0 && len(namespaces) > limit {
		extra := len(namespaces) - limit
		namespaces = namespaces[:limit]
		parts := make([]string, 0, len(namespaces)+1)
		for _, ns := range namespaces {
			parts = append(parts, fmt.Sprintf("%s(%d)", ns, counts[ns]))
		}
		parts = append(parts, fmt.Sprintf("+%d more", extra))
		return strings.Join(parts, ", ")
	}

	parts := make([]string, 0, len(namespaces))
	for _, ns := range namespaces {
		parts = append(parts, fmt.Sprintf("%s(%d)", ns, counts[ns]))
	}
	return strings.Join(parts, ", ")
}

// Cleanup removes temporary files created by the installer.
//
// Parameters:
//   - ctx: The context for the operation.
//   - p: *CommandParams containing configuration and runtime parameters.
//
// Returns:
//   - error: An error if cleanup fails, otherwise nil.
func Cleanup(ctx context.Context, p *CommandParams) error {
	log.Debug("Entering", "internal", "cleanup")
	defer log.Debug("Completed", "internal", "cleanup")

	tmp := p.Settings().Config.Tmp
	// Removing the working directory takes the generated kubeconfig with it.
	// After a teardown that is the correct, desired behaviour (a kubeconfig
	// pointing at a destroyed cluster is a footgun), but doing it silently is
	// surprising — operators have lost shells/redirects pointed there. Announce
	// it so the removal is never a mystery. Durable run logs live under log/,
	// not here, so nothing observable is lost.
	if _, statErr := os.Stat(tmp); statErr == nil {
		util.Msgf("Removing local working directory %s (generated kubeconfig and manifests)", tmp)
	}

	err := os.RemoveAll(tmp)
	if err != nil {
		log.Warn("Error during cleanup", "err", err)
	}
	return nil
}

// Banner displays the Quartz banner.
func Banner() {
	log.Debug("Entering", "internal", "banner")
	defer log.Debug("Completed", "internal", "banner")

	util.PrintBanner()
}

// Confirm prompts the user for confirmation before proceeding with an operation.
//
// Parameters:
//   - ctx: The context for the operation.
//   - msg: The confirmation message to display.
//   - p: *CommandParams containing configuration and runtime parameters.
//
// Returns:
//   - error: An error if the user does not confirm, otherwise nil.
func Confirm(ctx context.Context, msg string, p *CommandParams) error {
	log.Debug("Entering", "internal", "confirm")
	defer log.Debug("Completed", "internal", "confirm")

	if cp, err := p.Provider().Cloud(ctx); err != nil {
		log.Warn("Could not load cloud provider config for confirmation prompt", "error", err)
	} else if cp != nil {
		cp.PrintConfig()
	}

	util.Msgf("Domain: %s\n", p.Settings().Config.Dns.Domain)

	if p.assumeYes {
		util.Msg("Confirmation prompt skipped (--yes)")
		return nil
	}

	if r := util.PromptYesNo(msg); !r {
		return fmt.Errorf("aborting")
	}

	return nil
}

// Export saves the configured Kubernetes resources to YAML files.
//
// Parameters:
//   - ctx: The context for the operation.
//   - p: *CommandParams containing configuration and runtime parameters.
//
// Returns:
//   - error: An error if exporting resources fails, otherwise nil.
func Export(ctx context.Context, p *CommandParams) error {
	log.Debug("Entering", "command", "export")
	defer log.Debug("Completed", "command", "export")

	k8s, err := p.Provider().Kubernetes(ctx)
	if err != nil {
		return err
	}

	res, err := k8s.Export(ctx, p.Settings().Config.Export)
	if err != nil {
		return err
	}

	out := p.Settings().Config.Export.Path
	for k, v := range res {
		err := util.WriteBytesToFile(v, path.Join(out, p.Settings().Config.Dns.Domain, k))
		if err != nil {
			return err
		}
	}

	return nil
}

// PrepareAccount prepares the cloud account for Quartz operations.
//
// Parameters:
//   - ctx: The context for the operation.
//   - p: *CommandParams containing configuration and runtime parameters.
//
// Returns:
//   - error: An error if preparing the account fails, otherwise nil.
func PrepareAccount(ctx context.Context, p *CommandParams) error {
	log.Debug("Entering", "internal", "prepareAccount")
	defer log.Debug("Completed", "internal", "prepareAccount")

	cp, _ := p.Provider().Cloud(ctx)
	return cp.PrepareAccount(ctx)
}

// Restart restarts a Kubernetes resource in the specified namespace.
//
// Parameters:
//   - ctx: The context for the operation.
//   - res: The resource type to restart (e.g., deployment, daemonset).
//   - ns: The namespace of the resource.
//   - name: The name of the resource.
//   - p: *CommandParams containing configuration and runtime parameters.
//
// Returns:
//   - error: An error if restarting the resource fails, otherwise nil.
func Restart(ctx context.Context, res string, ns string, name string, p *CommandParams) error {
	k8s, err := p.Provider().Kubernetes(ctx)
	if err != nil {
		return err
	}

	kind, err := k8s.LookupKind(ctx, res)
	if err != nil {
		return err
	}

	return k8s.Restart(ctx, kind, ns, name)
}

// onCheckStart logs the start of a health check for a stage.
//
// Parameters:
//   - cr: The result of the health check.
func onCheckStart(cr stages.CheckResult) {
	util.Msgf("Starting %s check for stage %s - %s", cr.Type, cr.Stage, cr.Id)
}

// onCheckComplete logs the completion of a health check for a stage.
//
// Parameters:
//   - cr: The result of the health check.
func onCheckComplete(cr stages.CheckResult) {
	log.Debug("Health check complete", "result", cr)
	if cr.Error != nil {
		util.Errorf("Error running %s check for stage %s - %s [%v]", cr.Type, cr.Stage, cr.Id, cr.Error)
	} else {
		util.Msgf("Completed %s check for stage %s - %s", cr.Type, cr.Stage, cr.Id)
	}
}

// onCheckRetry logs a retry attempt for a health check.
// Provides enhanced diagnostic output including the error reason and periodic summaries.
//
// Parameters:
//   - cr: The result of the health check.
//   - i: The retry attempt number.
func onCheckRetry(cr stages.CheckResult, i int) {
	// Basic retry message with error reason
	errMsg := ""
	if cr.Error != nil {
		errMsg = fmt.Sprintf(" [%v]", cr.Error)
	}

	// For every 5th retry (or first), show more detailed message
	if i == 1 || i%5 == 0 {
		util.Printf("Waiting for %s check: %s - %s (attempt %d)%s", cr.Type, cr.Stage, cr.Id, i, errMsg)
	} else {
		// Shorter message for intermediate retries
		util.Printf("Retrying %s check for stage %s - %s (%d)", cr.Type, cr.Stage, cr.Id, i)
	}
}
