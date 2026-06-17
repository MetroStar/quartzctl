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

package provider

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	quartzSchema "github.com/MetroStar/quartzctl/internal/config/schema"
	"github.com/MetroStar/quartzctl/internal/log"
	"github.com/MetroStar/quartzctl/internal/util"

	authv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"kmodules.xyz/client-go/tools/wait"
	"sigs.k8s.io/yaml"

	"github.com/moby/spdystream"
)

var defaultCache = &KubernetesLookupCache{
	mutex: &sync.Mutex{},
	kinds: map[string]schema.GroupVersionResource{},
}

var helmActionTimeoutPattern = regexp.MustCompile(`(?i)timeout(?: of)? ([0-9][0-9a-zA-Z.]*)`)

var queryPrometheusViaPortForward = queryPrometheusViaPortForwardImpl
var suppressSpdyDebugOnce sync.Once

const prometheusPortForwardTimeout = 8 * time.Second

func helmActionTimeout(msg string) time.Duration {
	m := helmActionTimeoutPattern.FindStringSubmatch(msg)
	if m == nil {
		return 0
	}
	d, err := time.ParseDuration(m[1])
	if err != nil {
		return 0
	}
	return d
}

func logSnapshotFailure(action string, err error) {
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Debug("Progress snapshot: "+action+" failed (non-fatal)", "err", err)
	}
}

// KubernetesProviderClient defines the interface for Kubernetes provider clients.
type KubernetesProviderClient interface {
	Provider
	LookupKind(ctx context.Context, kind string) (schema.GroupVersionResource, error)
	WaitConditionState(ctx context.Context, kind schema.GroupVersionResource, ns string, name string, state string, timeoutSeconds int) error
	PrintClusterInfo(ctx context.Context)
	WriteKubeconfigFile(path string) error
	RefreshExternalSecrets(ctx context.Context) ([]KubernetesResource, error)
	Export(ctx context.Context, cfg quartzSchema.ExportConfig) (map[string][]byte, error)
	GetConfigMapValue(ctx context.Context, ns string, name string) (map[string]string, error)
	QueryPrometheus(ctx context.Context, ns string, service string, port int, query string) (PrometheusQueryResponse, error)
	GetCleanupStatus(ctx context.Context, ns string, name string) (CleanupStatus, error)
	GetSecretValue(ctx context.Context, ns string, name string) (map[string]string, error)
	Restart(ctx context.Context, kind schema.GroupVersionResource, ns string, name string) error
	GetDaemonSetStatus(ctx context.Context, kind schema.GroupVersionResource, ns string, name string) (int64, int64, error)
	CleanupStuckTerminatingPods(ctx context.Context, timeout time.Duration) ([]string, error)
	ReapOrphanedAdmissionWebhooks(ctx context.Context) ([]string, error)
	ScrubStuckHelmReleaseSecrets(ctx context.Context, minAge time.Duration) ([]string, error)
	AssessExternalSecretsTeardown(ctx context.Context) (ExternalSecretsTeardownAssessment, error)
	ClusterProgressSnapshot(ctx context.Context) (ClusterProgress, error)
	ListVirtualServices(ctx context.Context) ([]VirtualServiceInfo, error)
	PrepareForDestroy(ctx context.Context) error
	InterStageCleanup(ctx context.Context) error
}

// KubernetesClient is the implementation of the Kubernetes provider client.
type KubernetesClient struct {
	opts  KubeconfigInfo
	cfg   quartzSchema.QuartzConfig
	api   KubernetesApi
	cache *KubernetesLookupCache
}

// KubernetesLookupCache is a cache for Kubernetes resource kinds.
type KubernetesLookupCache struct {
	mutex *sync.Mutex
	kinds map[string]schema.GroupVersionResource
}

// KubeconfigInfo contains information about the Kubernetes configuration.
type KubeconfigInfo struct {
	Cluster              string
	Context              string
	User                 string
	Endpoint             string
	CertificateAuthority string
	Token                string
	Expiration           time.Time
}

// ExternalSecretsTeardownAssessment summarizes whether the external-secrets
// release has effectively been torn down even if Helm timed out deleting its
// own bookkeeping.
type ExternalSecretsTeardownAssessment struct {
	Recoverable        bool
	HelmReleaseSecrets []string
	RemainingResources []string
	Summary            string
}

// KubernetesAppConnectionInfo contains information about an application's connection in Kubernetes.
type KubernetesAppConnectionInfo struct {
	Name           string
	PublicEndpoint string
	AdminUsername  string
	AdminPassword  string
	Error          error
}

// CleanupStatus contains the best-effort status breadcrumbs emitted by the
// Quartz chart pre-delete hook. It is intentionally limited to non-secret
// metadata so teardown reports can be persisted safely.
type CleanupStatus struct {
	Data       map[string]string
	Events     []CleanupEvent
	HookEvents []CleanupHookEvent
}

// CleanupHookEvent is one cumulative lifecycle breadcrumb appended by the
// Quartz pre-delete hook into the cleanup status ConfigMap.
type CleanupHookEvent struct {
	At     time.Time `json:"at"`
	Kind   string    `json:"kind"`
	Phase  string    `json:"phase"`
	Status string    `json:"status"`
	Detail string    `json:"detail"`
}

// CleanupEvent is a compact, stable view of a Kubernetes Event emitted by the
// Quartz cleanup hook.
type CleanupEvent struct {
	Reason        string
	Type          string
	Message       string
	Count         int32
	LastTimestamp time.Time
}

// KubernetesResource represents a Kubernetes resource.
type KubernetesResource struct {
	Name      string
	Namespace string
	Kind      schema.GroupVersionResource
	Item      unstructured.Unstructured
}

// VirtualServiceInfo contains information about a VirtualService.
type VirtualServiceInfo struct {
	Name      string
	Namespace string
	Hosts     []string
	Gateways  []string
}

type PrometheusQueryResponse struct {
	Status    string              `json:"status"`
	Data      PrometheusQueryData `json:"data"`
	ErrorType string              `json:"errorType,omitempty"`
	Error     string              `json:"error,omitempty"`
}

type PrometheusQueryData struct {
	ResultType string                  `json:"resultType"`
	Result     []PrometheusQueryResult `json:"result"`
}

type PrometheusQueryResult struct {
	Metric map[string]string `json:"metric"`
	Value  []any             `json:"value"`
}

func (r PrometheusQueryResponse) ScalarValue() (float64, bool) {
	if len(r.Data.Result) == 0 || len(r.Data.Result[0].Value) < 2 {
		return 0, false
	}

	switch v := r.Data.Result[0].Value[1].(type) {
	case string:
		f, err := strconv.ParseFloat(v, 64)
		return f, err == nil
	case float64:
		return v, true
	case int:
		return float64(v), true
	default:
		return 0, false
	}
}

type KubernetesProviderCheckResult struct {
	Status bool
	Error  error

	cfg quartzSchema.QuartzConfig
}

// NewKubernetesClient creates a new KubernetesClient instance.
func NewKubernetesClient(api KubernetesApi, kubeconfig KubeconfigInfo, cfg quartzSchema.QuartzConfig) (KubernetesClient, error) {
	c := KubernetesClient{
		opts:  kubeconfig,
		cfg:   cfg,
		api:   api,
		cache: defaultCache,
	}

	return c, nil
}

// ProviderName returns the name of the provider.
func (c KubernetesClient) ProviderName() string {
	return "Kubernetes"
}

// CheckAccess checks access to the Kubernetes cluster.
func (c KubernetesClient) CheckAccess(ctx context.Context) ProviderCheckResult {
	// just do a simple lookup to see if we can access the cluster
	cs, err := c.api.ClientSet()
	if err == nil {
		_, err = cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	}

	return KubernetesProviderCheckResult{
		Status: err == nil,
		Error:  err,
		cfg:    c.cfg,
	}
}

// EnsureKubeconfig ensures that the kubeconfig file exists at the specified path.
func (c KubernetesClient) EnsureKubeconfig(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return c.WriteKubeconfigFile(path)
	}

	return nil
}

// WriteKubeconfigFile writes the kubeconfig to the specified file path.
func (c KubernetesClient) WriteKubeconfigFile(path string) error {
	log.Debug("Writing kubeconfig", "path", path)

	f, err := os.Create(path) // #nosec G304
	if err != nil {
		return err
	}
	defer f.Close()

	return c.WriteKubeconfig(f)
}

// WriteKubeconfig writes the kubeconfig to the provided writer.
func (c KubernetesClient) WriteKubeconfig(w io.Writer) error {
	data := c.opts.ToKubeconfigYamlBytes(c.cfg)
	_, err := w.Write(data)
	if err != nil {
		return err
	}

	return nil
}

// PrintClusterInfo prints information about the cluster and its applications.
func (c KubernetesClient) PrintClusterInfo(ctx context.Context) {
	apps := map[string]quartzSchema.ApplicationLookupConfig{}
	configuredIngressNames := make(map[string]bool)

	for k, v := range c.cfg.Core.Applications {
		if v.Disabled {
			continue
		}

		if !v.Lookup.Enabled {
			continue
		}

		name := v.Description
		if name == "" {
			name = k
		}
		apps[name] = v.Lookup

		// Track configured ingress names to exclude from discovery
		if v.Lookup.Ingress.Name != "" {
			configuredIngressNames[v.Lookup.Ingress.Name] = true
		}
	}
	c.PrintClusterAppInfo(ctx, apps)

	// Print additional discovered VirtualServices
	c.PrintDiscoveredVirtualServices(ctx, configuredIngressNames)
}

// PrintDiscoveredVirtualServices prints VirtualServices that are not in the configured applications.
func (c KubernetesClient) PrintDiscoveredVirtualServices(ctx context.Context, excludeNames map[string]bool) {
	virtualServices, err := c.ListVirtualServices(ctx)
	if err != nil {
		log.Debug("Failed to list VirtualServices", "error", err)
		return
	}

	var rows [][]string
	for _, vs := range virtualServices {
		// Skip VirtualServices that are in the configured apps
		if excludeNames[vs.Name] {
			continue
		}

		// Skip mesh-only gateways (internal services)
		isMeshOnly := true
		for _, gw := range vs.Gateways {
			if gw != "mesh" {
				isMeshOnly = false
				break
			}
		}
		if isMeshOnly && len(vs.Gateways) > 0 {
			continue
		}

		// Get the first host for display
		host := ""
		if len(vs.Hosts) > 0 {
			host = vs.Hosts[0]
		}

		rows = append(rows, []string{vs.Name, vs.Namespace, fmt.Sprintf("https://%s", host)})
	}

	if len(rows) == 0 {
		return
	}

	fmt.Println() // Add spacing
	util.Printf("Additional Services")
	headers := []string{"Name", "Namespace", "URL"}
	util.PrintTable(headers, rows)
}

// PrintClusterAppInfo prints detailed information about the specified applications in the cluster.
func (c KubernetesClient) PrintClusterAppInfo(ctx context.Context, apps map[string]quartzSchema.ApplicationLookupConfig) {
	ch := make(chan KubernetesAppConnectionInfo, len(apps))

	tctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	for k, v := range apps {
		wg.Add(1)
		app := k
		opts := v
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					ch <- KubernetesAppConnectionInfo{
						Name:  app,
						Error: fmt.Errorf("panic: %v", r),
					}
				}
			}()

			select {
			case ch <- c.GetAppConnectionInfo(tctx, app, opts):
			case <-tctx.Done():
				// Context canceled, don't block trying to send
			}
		}()
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	var rows [][]string
	hasError := false

	for range apps {
		i := <-ch

		if i.Error != nil {
			hasError = true
			rows = append(rows, []string{i.Name, fmt.Sprintf("https://%s", i.PublicEndpoint), i.AdminUsername, i.AdminPassword, i.Error.Error()})
			continue
		}

		rows = append(rows, []string{i.Name, fmt.Sprintf("https://%s", i.PublicEndpoint), i.AdminUsername, i.AdminPassword})
	}

	headers := []string{"Application", "URL", "Admin User", "Admin Password"}
	if hasError {
		headers = append(headers, "Error")
	}

	// sort rows by application name for consistent ordering
	slices.SortFunc(rows, func(lhs []string, rhs []string) int {
		return cmp.Compare(lhs[0], rhs[0])
	})

	// Admin passwords are intentionally visible in the operator's terminal, but
	// the sensitive-table path keeps the logger from ever recording cell values.
	util.PrintSensitiveTable(headers, rows, "Admin Password")
}

// RefreshExternalSecrets triggers a refresh of external secrets in the cluster.
func (c KubernetesClient) RefreshExternalSecrets(ctx context.Context) ([]KubernetesResource, error) {
	// https://external-secrets.io/latest/introduction/faq/#can-i-manually-trigger-a-secret-refresh
	kind, err := c.LookupKind(ctx, "ExternalSecret")
	if err != nil {
		return nil, err
	}

	var result []KubernetesResource

	timestamp := time.Now().UTC().Format(time.RFC3339)
	err = c.ForEachDynamicResources(ctx, kind, "", func(item unstructured.Unstructured) {
		name := item.GetName()
		ns := item.GetNamespace()

		util.Printf("Triggering refresh of secret %s/%s", ns, name)

		annotations := item.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}
		annotations["force-sync"] = timestamp
		item.SetAnnotations(annotations)
		_, ierr := c.Update(ctx, kind, ns, &item)
		if ierr != nil {
			log.Info("Error updating dynamic resource", "name", name, "ns", ns, "err", ierr)
		}

		result = append(result, KubernetesResource{
			Name:      name,
			Namespace: ns,
			Kind:      kind,
			Item:      item,
		})
	})

	return result, err
}

// ListVirtualServices returns all VirtualServices in the cluster with their hosts and gateways.
func (c KubernetesClient) ListVirtualServices(ctx context.Context) (result []VirtualServiceInfo, err error) {
	// Recover from panics (e.g., during tests with fake clients that don't have all kinds registered)
	defer func() {
		if r := recover(); r != nil {
			log.Debug("Recovered from panic in ListVirtualServices", "panic", r)
			err = fmt.Errorf("failed to list VirtualServices: %v", r)
		}
	}()

	kind, lookupErr := c.LookupKind(ctx, "VirtualService")
	if lookupErr != nil {
		return nil, fmt.Errorf("failed to lookup VirtualService kind: %w", lookupErr)
	}

	listErr := c.ForEachDynamicResources(ctx, kind, "", func(item unstructured.Unstructured) {
		name := item.GetName()
		ns := item.GetNamespace()

		hosts, _, _ := unstructured.NestedStringSlice(item.Object, "spec", "hosts")
		gateways, _, _ := unstructured.NestedStringSlice(item.Object, "spec", "gateways")

		result = append(result, VirtualServiceInfo{
			Name:      name,
			Namespace: ns,
			Hosts:     hosts,
			Gateways:  gateways,
		})
	})

	if listErr != nil {
		return nil, fmt.Errorf("failed to list VirtualServices: %w", listErr)
	}

	// Sort by namespace/name for consistent ordering
	slices.SortFunc(result, func(a, b VirtualServiceInfo) int {
		if a.Namespace != b.Namespace {
			return cmp.Compare(a.Namespace, b.Namespace)
		}
		return cmp.Compare(a.Name, b.Name)
	})

	return result, nil
}

// Export exports Kubernetes resources based on the provided configuration.
func (c KubernetesClient) Export(ctx context.Context, cfg quartzSchema.ExportConfig) (map[string][]byte, error) {
	res := make(map[string][]byte)
	errs := []error{}

	for _, s := range cfg.Objects {
		util.Printf("Export %s %s/%s", s.Kind, s.Namespace, s.Name)

		k, err := c.LookupKind(ctx, s.Kind)
		if err != nil {
			// TODO: log
			errs = append(errs, err)
			continue
		}

		o, err := c.GetDynamicResource(ctx, k, s.Namespace, s.Name)
		if err != nil {
			// TODO: log
			errs = append(errs, err)
			continue
		}

		for k, v := range cfg.Annotations {
			err = unstructured.SetNestedField(o, v, "metadata", "annotations", k)
			if err != nil {
				errs = append(errs, err)
			}
		}

		y, err := yaml.Marshal(o)
		if err != nil {
			// TODO: log
			errs = append(errs, err)
			continue
		}

		res[fmt.Sprintf("%s.%s.yaml", s.Namespace, s.Name)] = y
	}

	return res, errors.Join(errs...)
}

// WaitConditionState waits for a resource to reach a specific condition state.
func (c KubernetesClient) WaitConditionState(ctx context.Context, kind schema.GroupVersionResource, ns string, name string, state string, timeoutSeconds int) error {
	client, _ := c.api.DynamicClient()

	f := []*resource.Info{
		{
			Mapping: &meta.RESTMapping{
				Resource: kind,
			},
			Name:      name,
			Namespace: ns,
		},
	}
	cf, err := wait.ConditionFuncFor(fmt.Sprintf("condition=%s", state), io.Discard)
	if err != nil {
		return err
	}

	t := timeoutSeconds
	if t <= 0 {
		// default timeout if not specified, 10 minutes
		t = 600
	}
	o := &wait.WaitOptions{
		ResourceFinder: genericclioptions.NewSimpleFakeResourceFinder(f...),
		DynamicClient:  client,
		Timeout:        time.Duration(t) * time.Second,

		Printer:     printers.NewDiscardingPrinter(),
		ConditionFn: cf,
		IOStreams:   genericclioptions.NewTestIOStreamsDiscard(),
	}

	err = o.RunWait()

	return err
}

// GetConfigMapValue retrieves the key-value pairs from a ConfigMap.
func (c KubernetesClient) GetConfigMapValue(ctx context.Context, ns string, name string) (map[string]string, error) {
	clientset, err := c.api.ClientSet()
	if err != nil {
		return nil, err
	}

	cms := clientset.CoreV1().ConfigMaps(ns)
	cm, err := cms.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	res := make(map[string]string)
	for k, v := range cm.Data {
		res[k] = v
	}

	return res, nil
}

func (c KubernetesClient) QueryPrometheus(ctx context.Context, ns string, service string, port int, query string) (PrometheusQueryResponse, error) {
	clientset, err := c.api.ClientSet()
	if err != nil {
		return PrometheusQueryResponse{}, err
	}

	name := fmt.Sprintf("http:%s:%d", service, port)
	raw, err := clientset.CoreV1().RESTClient().
		Get().
		Namespace(ns).
		Resource("services").
		Name(name).
		SubResource("proxy").
		Suffix("api", "v1", "query").
		Param("query", query).
		DoRaw(ctx)
	if err != nil {
		fallbackCtx, cancel := prometheusPortForwardContext(ctx)
		defer cancel()

		res, pfErr := queryPrometheusViaPortForward(fallbackCtx, c.api, clientset, ns, service, port, query)
		if pfErr == nil {
			return res, nil
		}
		return PrometheusQueryResponse{}, fmt.Errorf("%w; port-forward fallback also failed: %v", err, pfErr)
	}

	var res PrometheusQueryResponse
	if err := json.Unmarshal(raw, &res); err != nil {
		return PrometheusQueryResponse{}, err
	}
	if res.Status != "" && res.Status != "success" {
		return res, fmt.Errorf("prometheus query failed: %s %s", res.ErrorType, res.Error)
	}

	return res, nil
}

func prometheusPortForwardContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) > time.Second {
		return ctx, func() {}
	}
	return context.WithTimeout(context.WithoutCancel(ctx), prometheusPortForwardTimeout)
}

func queryPrometheusViaPortForwardImpl(ctx context.Context, api KubernetesApi, clientset kubernetes.Interface, ns string, service string, port int, query string) (PrometheusQueryResponse, error) {
	podName, err := prometheusProxyPodName(ctx, clientset, ns, service, port)
	if err != nil {
		return PrometheusQueryResponse{}, err
	}

	rc := api.RESTConfig()
	if rc == nil {
		return PrometheusQueryResponse{}, fmt.Errorf("no Kubernetes REST config available for Prometheus port-forward fallback")
	}

	transport, upgrader, err := spdy.RoundTripperFor(rest.CopyConfig(rc))
	if err != nil {
		return PrometheusQueryResponse{}, err
	}

	serverURL, err := url.Parse(rc.Host)
	if err != nil {
		return PrometheusQueryResponse{}, err
	}
	serverURL.Path = fmt.Sprintf("/api/v1/namespaces/%s/pods/%s/portforward", ns, podName)

	suppressSpdyDebugLogs()

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, http.MethodPost, serverURL)
	stopCh := make(chan struct{})
	readyCh := make(chan struct{})
	defer close(stopCh)

	var stdout, stderr bytes.Buffer
	fw, err := portforward.New(dialer, []string{fmt.Sprintf("0:%d", port)}, stopCh, readyCh, &stdout, &stderr)
	if err != nil {
		return PrometheusQueryResponse{}, err
	}

	forwardErrCh := make(chan error, 1)
	go func() {
		forwardErrCh <- fw.ForwardPorts()
	}()

	select {
	case <-readyCh:
	case err := <-forwardErrCh:
		if strings.TrimSpace(stderr.String()) != "" {
			return PrometheusQueryResponse{}, fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
		}
		return PrometheusQueryResponse{}, err
	case <-ctx.Done():
		return PrometheusQueryResponse{}, ctx.Err()
	}

	ports, err := fw.GetPorts()
	if err != nil {
		return PrometheusQueryResponse{}, err
	}
	if len(ports) == 0 {
		return PrometheusQueryResponse{}, fmt.Errorf("Prometheus port-forward returned no local ports")
	}

	reqURL := fmt.Sprintf("http://127.0.0.1:%d/api/v1/query?query=%s", ports[0].Local, url.QueryEscape(query))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return PrometheusQueryResponse{}, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return PrometheusQueryResponse{}, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		if trimmed := strings.TrimSpace(string(body)); trimmed != "" {
			return PrometheusQueryResponse{}, fmt.Errorf("port-forward query returned HTTP %d: %s", resp.StatusCode, trimmed)
		}
		return PrometheusQueryResponse{}, fmt.Errorf("port-forward query returned HTTP %d", resp.StatusCode)
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return PrometheusQueryResponse{}, err
	}

	var res PrometheusQueryResponse
	if err := json.Unmarshal(raw, &res); err != nil {
		return PrometheusQueryResponse{}, err
	}
	if res.Status != "" && res.Status != "success" {
		return res, fmt.Errorf("prometheus query failed: %s %s", res.ErrorType, res.Error)
	}

	return res, nil
}

func prometheusProxyPodName(ctx context.Context, clientset kubernetes.Interface, ns string, service string, port int) (string, error) {
	slices, err := clientset.DiscoveryV1().EndpointSlices(ns).List(ctx, metav1.ListOptions{
		LabelSelector: "kubernetes.io/service-name=" + service,
	})
	if err != nil {
		return "", err
	}

	var fallbackPod string
	for _, slice := range slices.Items {
		portMatch := len(slice.Ports) == 0
		for _, slicePort := range slice.Ports {
			if slicePort.Port != nil && int(*slicePort.Port) == port {
				portMatch = true
				break
			}
		}
		if !portMatch && len(slice.Ports) > 0 {
			continue
		}

		for _, endpoint := range slice.Endpoints {
			if endpoint.TargetRef == nil || !strings.EqualFold(endpoint.TargetRef.Kind, "Pod") || strings.TrimSpace(endpoint.TargetRef.Name) == "" {
				continue
			}
			if portMatch {
				return endpoint.TargetRef.Name, nil
			}
			if fallbackPod == "" {
				fallbackPod = endpoint.TargetRef.Name
			}
		}
	}

	if fallbackPod != "" {
		return fallbackPod, nil
	}

	return "", fmt.Errorf("service %s/%s has no pod-backed endpoint slice for port %d", ns, service, port)
}

func suppressSpdyDebugLogs() {
	suppressSpdyDebugOnce.Do(func() {
		spdystream.DEBUG = ""
	})
}

// GetCleanupStatus retrieves non-secret cleanup breadcrumbs emitted by the
// Quartz chart pre-delete hook.
func (c KubernetesClient) GetCleanupStatus(ctx context.Context, ns string, name string) (CleanupStatus, error) {
	clientset, err := c.api.ClientSet()
	if err != nil {
		return CleanupStatus{}, err
	}

	cm, err := clientset.CoreV1().ConfigMaps(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return CleanupStatus{}, err
	}

	status := CleanupStatus{Data: make(map[string]string, len(cm.Data))}
	for k, v := range cm.Data {
		status.Data[k] = v
	}
	if raw := strings.TrimSpace(status.Data["cleanupEvents"]); raw != "" {
		if err := json.Unmarshal([]byte(raw), &status.HookEvents); err != nil {
			log.Debug("Cleanup status history parse failed (non-fatal)", "namespace", ns, "name", name, "error", err)
		} else {
			slices.SortFunc(status.HookEvents, func(a, b CleanupHookEvent) int {
				if c := a.At.Compare(b.At); c != 0 {
					return c
				}
				if c := cmp.Compare(a.Phase, b.Phase); c != 0 {
					return c
				}
				return cmp.Compare(a.Detail, b.Detail)
			})
		}
	}

	events, err := clientset.CoreV1().Events(ns).List(ctx, metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=quartz-cleanup",
	})
	if err != nil {
		log.Debug("Cleanup status event lookup failed (non-fatal)", "namespace", ns, "error", err)
		return status, nil
	}

	for _, e := range events.Items {
		status.Events = append(status.Events, CleanupEvent{
			Reason:        e.Reason,
			Type:          e.Type,
			Message:       e.Message,
			Count:         e.Count,
			LastTimestamp: e.LastTimestamp.Time,
		})
	}
	slices.SortFunc(status.Events, func(a, b CleanupEvent) int {
		if c := a.LastTimestamp.Compare(b.LastTimestamp); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Reason, b.Reason); c != 0 {
			return c
		}
		return cmp.Compare(a.Message, b.Message)
	})

	return status, nil
}

// GetSecret retrieves a Secret from the cluster.
func (c KubernetesClient) GetSecret(ctx context.Context, ns string, name string) (*corev1.Secret, error) {
	clientset, err := c.api.ClientSet()
	if err != nil {
		return nil, err
	}

	secrets := clientset.CoreV1().Secrets(ns)
	return secrets.Get(ctx, name, metav1.GetOptions{})
}

// GetSecretValue retrieves the key-value pairs from a Secret.
func (c KubernetesClient) GetSecretValue(ctx context.Context, ns string, name string) (map[string]string, error) {
	s, err := c.GetSecret(ctx, ns, name)
	if err != nil {
		return nil, err
	}

	res := make(map[string]string)
	for k, v := range s.Data {
		res[k] = string(v)
	}

	return res, nil
}

// GetAppConnectionInfo retrieves connection information for an application.
func (c KubernetesClient) GetAppConnectionInfo(ctx context.Context, name string, opts quartzSchema.ApplicationLookupConfig) KubernetesAppConnectionInfo {
	res := KubernetesAppConnectionInfo{
		Name: name,
	}
	var errs []error

	if opts.AdminCredentials.Secret.Name != "" {
		credentials, err := c.GetSecretValue(ctx, opts.AdminCredentials.Secret.Namespace, opts.AdminCredentials.Secret.Name)
		if err != nil {
			errs = append(errs, err)
		} else {
			username, ok := credentials[opts.AdminCredentials.Secret.UsernameKey]
			if !ok {
				username = opts.AdminCredentials.Username
			}
			res.AdminUsername = username

			res.AdminPassword = credentials[opts.AdminCredentials.Secret.PasswordKey]
		}
	} else {
		log.Debug("No admin credentials secret provided", "app", name)
	}

	if opts.Ingress.Name != "" {
		var ingressKind schema.GroupVersionResource
		if opts.Ingress.Kind != "" &&
			opts.Ingress.Group != "" &&
			opts.Ingress.Version != "" {
			ingressKind = schema.GroupVersionResource{
				Group:    opts.Ingress.Group,
				Version:  opts.Ingress.Version,
				Resource: opts.Ingress.Kind,
			}
		} else {
			ingressKind, _ = c.LookupKind(ctx, opts.Ingress.Kind)
		}

		if ingressKind.Empty() {
			errs = append(errs, fmt.Errorf("ingress kind not found, %v", opts.Ingress.Kind))
		}

		vs, err := c.GetDynamicResource(ctx, ingressKind, opts.Ingress.Namespace, opts.Ingress.Name)
		if err != nil {
			errs = append(errs, err)
		}

		hosts, found, err := unstructured.NestedStringSlice(vs, "spec", "hosts")
		if err != nil {
			errs = append(errs, err)
		} else if !found {
			errs = append(errs, fmt.Errorf("ingress not found for %s", name))
		} else {
			res.PublicEndpoint = hosts[0]
		}
	} else {
		log.Debug("No ingress provided", "app", name)
	}

	if len(errs) > 0 {
		res.Error = errors.Join(errs...)
	}

	return res
}

// LookupKind looks up the GroupVersionResource for a given kind.
func (c KubernetesClient) LookupKind(ctx context.Context, kind string) (schema.GroupVersionResource, error) {
	c.cache.mutex.Lock()
	defer c.cache.mutex.Unlock()

	cached, ok := c.cache.kinds[kind]
	if ok {
		return cached, nil
	}

	dc, err := c.api.DiscoveryClient()
	if err != nil {
		return schema.GroupVersionResource{}, err
	}

	groupResources, err := restmapper.GetAPIGroupResources(dc)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}

	disc := restmapper.NewDiscoveryRESTMapper(groupResources)
	mapper := restmapper.NewShortcutExpander(disc, dc, func(msg string) {
		log.Warn("Unexpected shortcut expander warning", "message", msg)
	})

	// try to parse as a shortcut
	rs, err := mapper.ResourcesFor(schema.GroupVersionResource{Resource: kind})
	if err == nil && len(rs) > 0 {
		log.Debug("Found", "kind", kind, "gvr", rs[0])
		c.cache.kinds[kind] = rs[0]
		return rs[0], nil
	}

	// try again assuming fully qualified type.group kind
	k := schema.ParseGroupKind(kind)
	mapping, err := mapper.RESTMapping(k)
	if err == nil {
		log.Debug("Found gvr %v for kind %s", mapping.Resource, kind)
		c.cache.kinds[kind] = mapping.Resource
		return mapping.Resource, nil
	}

	return schema.GroupVersionResource{}, err
}

// GetDynamicResource retrieves a dynamic resource from the cluster.
func (c KubernetesClient) GetDynamicResource(ctx context.Context, kind schema.GroupVersionResource, ns string, name string) (map[string]interface{}, error) {
	dyn, err := c.api.DynamicClient()
	if err != nil {
		return nil, err
	}

	i := dyn.Resource(kind)
	res, err := i.Namespace(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	return res.Object, nil
}

// CleanupStuckTerminatingPods force-deletes pods that have been stuck in Terminating
// state for longer than the specified timeout. This handles scenarios where pods
// cannot terminate gracefully due to CNI issues or other infrastructure problems.
func (c KubernetesClient) CleanupStuckTerminatingPods(ctx context.Context, timeout time.Duration) ([]string, error) {
	clientset, err := c.api.ClientSet()
	if err != nil {
		return nil, err
	}

	pods, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var cleaned []string
	gracePeriod := int64(0)
	deleteOpts := metav1.DeleteOptions{GracePeriodSeconds: &gracePeriod}

	for _, pod := range pods.Items {
		// Check if pod is terminating (has a deletionTimestamp)
		if pod.DeletionTimestamp == nil {
			continue
		}

		// Check if pod has been terminating longer than timeout
		terminatingDuration := time.Since(pod.DeletionTimestamp.Time)
		if terminatingDuration < timeout {
			continue
		}

		log.Info("Force-deleting stuck terminating pod",
			"namespace", pod.Namespace,
			"name", pod.Name,
			"terminating_for", terminatingDuration.Round(time.Second).String())

		err := clientset.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, deleteOpts)
		if err != nil {
			log.Warn("Failed to force-delete pod", "namespace", pod.Namespace, "name", pod.Name, "err", err)
			continue
		}

		cleaned = append(cleaned, fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))
	}

	return cleaned, nil
}

// HelmReleaseStatus is a per-release readiness detail captured during a
// ClusterProgressSnapshot. It carries enough signal for a convergence gate to
// distinguish a release that is still progressing (keep waiting) from one Flux
// has given up on (Stalled — fail fast and surface the reason).
type HelmReleaseStatus struct {
	Namespace  string
	Name       string
	Ready      bool
	ReadyMsg   string
	Stalled    bool
	StalledMsg string
	// Timeout is the release's own spec.timeout (the budget Flux gives a single
	// install/upgrade attempt before it fails and remediates). Zero when unset.
	// The convergence gate uses the largest of these to size its overall wait so
	// it never gives up on a release that is still inside its own allotted time.
	Timeout time.Duration
}

// ID returns the "namespace/name" identifier for the release.
func (h HelmReleaseStatus) ID() string {
	return fmt.Sprintf("%s/%s", h.Namespace, h.Name)
}

// ClusterProgress is a point-in-time snapshot of cluster health used to give
// users visibility into a deploying Quartz cluster. It surfaces the signals
// that otherwise only become apparent minutes into a stalled install.
type ClusterProgress struct {
	HelmReleasesReady     int
	HelmReleasesTotal     int
	NotReadyReleases      []string
	Releases              []HelmReleaseStatus
	TerminatingNamespaces []string
	UnhealthyPods         []string
}

// NotReadyDetails returns the per-release detail for every release that is not
// currently Ready, in the order collected.
func (p ClusterProgress) NotReadyDetails() []HelmReleaseStatus {
	var out []HelmReleaseStatus
	for _, r := range p.Releases {
		if !r.Ready {
			out = append(out, r)
		}
	}
	return out
}

// StalledReleases returns the per-release detail for every release Flux has
// marked Stalled (retries/remediation exhausted), i.e. genuinely stuck.
func (p ClusterProgress) StalledReleases() []HelmReleaseStatus {
	var out []HelmReleaseStatus
	for _, r := range p.Releases {
		if r.Stalled {
			out = append(out, r)
		}
	}
	return out
}

// MaxReleaseTimeout returns the largest per-release spec.timeout in the
// snapshot (zero if none declare one). The convergence gate uses this to size
// its overall wait: a release legitimately allowed, say, 90m must not be
// declared a failure by a gate that only waits 30m.
func (p ClusterProgress) MaxReleaseTimeout() time.Duration {
	var max time.Duration
	for _, r := range p.Releases {
		if r.Timeout > max {
			max = r.Timeout
		}
	}
	return max
}

// Summary renders a concise one-line health summary suitable for periodic
// logging during an install.
func (p ClusterProgress) Summary() string {
	parts := []string{fmt.Sprintf("HelmReleases %d/%d ready", p.HelmReleasesReady, p.HelmReleasesTotal)}
	if n := len(p.UnhealthyPods); n > 0 {
		parts = append(parts, fmt.Sprintf("%d unhealthy pod(s)", n))
	}
	if n := len(p.TerminatingNamespaces); n > 0 {
		parts = append(parts, fmt.Sprintf("terminating ns: %s", strings.Join(p.TerminatingNamespaces, ",")))
	}
	return strings.Join(parts, " | ")
}

// ClusterProgressSnapshot collects a lightweight health snapshot of the cluster:
// HelmRelease readiness, namespaces stuck Terminating, and unhealthy pods. It is
// read-only and best-effort — individual collection failures degrade gracefully
// rather than returning an error, so it is safe to call repeatedly from a
// background reporter while a cluster is still coming up.
func (c KubernetesClient) ClusterProgressSnapshot(ctx context.Context) (ClusterProgress, error) {
	var progress ClusterProgress

	clientset, err := c.api.ClientSet()
	if err != nil {
		return progress, err
	}

	// HelmRelease readiness (Flux). Absent CRD / no releases yet is not an error.
	// Use the "HelmRelease" kind token (not the "helmreleases" resource name):
	// the kind form resolves reliably through the discovery RESTMapper, matching
	// the path used by stage checks, whereas the bare plural can fail to map.
	if gvr, lkErr := c.LookupKind(ctx, "HelmRelease"); lkErr == nil {
		ferr := c.ForEachDynamicResources(ctx, gvr, "", func(item unstructured.Unstructured) {
			progress.HelmReleasesTotal++
			conds, found, _ := unstructured.NestedSlice(item.Object, "status", "conditions")
			detail := HelmReleaseStatus{Namespace: item.GetNamespace(), Name: item.GetName()}
			// spec.timeout is a Go duration string (e.g. "45m", "90m"). Used by
			// the convergence gate to size its wait to the slowest release.
			if ts, ok, _ := unstructured.NestedString(item.Object, "spec", "timeout"); ok && ts != "" {
				if d, perr := time.ParseDuration(ts); perr == nil {
					detail.Timeout = d
				}
			}
			if found {
				for _, cond := range conds {
					m, ok := cond.(map[string]interface{})
					if !ok {
						continue
					}
					ctype, _ := m["type"].(string)
					cstatus, _ := m["status"].(string)
					cmsg, _ := m["message"].(string)
					if d := helmActionTimeout(cmsg); d > detail.Timeout {
						detail.Timeout = d
					}
					switch ctype {
					case "Ready":
						detail.Ready = cstatus == "True"
						detail.ReadyMsg = cmsg
					case "Stalled":
						// Flux's runtime sets the Stalled condition (status True)
						// when a release has hit a terminal error and retries are
						// exhausted — i.e. it will not converge without
						// intervention. This is the fail-fast signal for the
						// convergence gate.
						if cstatus == "True" {
							detail.Stalled = true
							detail.StalledMsg = cmsg
						}
					}
				}
			}
			progress.Releases = append(progress.Releases, detail)
			if detail.Ready {
				progress.HelmReleasesReady++
			} else {
				progress.NotReadyReleases = append(progress.NotReadyReleases,
					detail.ID())
			}
		})
		// Unlike a missing CRD (LookupKind failure above, which legitimately
		// means "no releases yet" → 0/0), a failure to LIST after a successful
		// kind lookup means the snapshot is unreliable (e.g. context canceled
		// when the reporter is stopped mid-call, or a transient API error).
		// Propagate it so the caller can skip emitting a misleading "0/0 ready"
		// instead of reporting partial counts as if they were authoritative.
		if ferr != nil {
			// A canceled context is expected when a progress reporter is stopped.
			logSnapshotFailure("listing HelmReleases", ferr)
			return progress, ferr
		}
	}

	// Namespaces stuck Terminating (a teardown/prune signal).
	if nsList, nsErr := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{}); nsErr == nil {
		for _, ns := range nsList.Items {
			if ns.Status.Phase == corev1.NamespaceTerminating {
				progress.TerminatingNamespaces = append(progress.TerminatingNamespaces, ns.Name)
			}
		}
	} else {
		logSnapshotFailure("listing namespaces", nsErr)
	}

	// Unhealthy pods: not Running/Succeeded, or stuck in a failing waiting state.
	if pods, podErr := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{}); podErr == nil {
		for _, pod := range pods.Items {
			if podUnhealthy(pod) {
				progress.UnhealthyPods = append(progress.UnhealthyPods,
					fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))
			}
		}
	} else {
		logSnapshotFailure("listing pods", podErr)
	}

	return progress, nil
}

// podUnhealthy reports whether a pod is in a state worth surfacing to the user.
// Pods that are terminating as part of normal node lifecycle are ignored.
func podUnhealthy(pod corev1.Pod) bool {
	if pod.DeletionTimestamp != nil {
		// Terminating pods are transient churn, not an install blocker.
		return false
	}
	switch pod.Status.Phase {
	case corev1.PodSucceeded, corev1.PodRunning:
		// Running pods can still be unhealthy if a container is wedged waiting.
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.State.Waiting != nil && isFailingWaitReason(cs.State.Waiting.Reason) {
				return true
			}
		}
		return false
	default:
		// Pending pods are normal briefly; flag only failing waiting reasons.
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.State.Waiting != nil && isFailingWaitReason(cs.State.Waiting.Reason) {
				return true
			}
		}
		for _, cs := range pod.Status.InitContainerStatuses {
			if cs.State.Waiting != nil && isFailingWaitReason(cs.State.Waiting.Reason) {
				return true
			}
		}
		return pod.Status.Phase == corev1.PodFailed
	}
}

// isFailingWaitReason reports whether a container waiting reason indicates a
// genuine failure rather than normal startup churn.
func isFailingWaitReason(reason string) bool {
	switch reason {
	case "CrashLoopBackOff", "ImagePullBackOff", "ErrImagePull",
		"CreateContainerConfigError", "CreateContainerError", "InvalidImageName":
		return true
	default:
		return false
	}
}

// ReapOrphanedAdmissionWebhooks deletes Validating/MutatingWebhookConfigurations
// whose backing Service no longer exists. Such "dangling" webhooks are a common
// failure mode for admission controllers (e.g. Kyverno, cert-manager) that create
// their webhook configurations dynamically at runtime: when the controller's Helm
// release is uninstalled, the Deployment/Service/namespace are removed but the
// cluster-scoped webhook configurations are left behind. With failurePolicy: Fail,
// every admission call then targets a dead service and fails, deadlocking ALL
// resource creation cluster-wide (no new pods, no Helm reconciles, even the
// controller cannot be reinstalled).
//
// A webhook is only reaped when its backing Service returns NotFound — i.e. the
// operator is already gone, so there is no live policy enforcement to preserve.
// Transient API errors are ignored so a brief apiserver hiccup never removes a
// healthy webhook. Returns the names of the webhook configurations that were
// removed.
func (c KubernetesClient) ReapOrphanedAdmissionWebhooks(ctx context.Context) ([]string, error) {
	clientset, err := c.api.ClientSet()
	if err != nil {
		return nil, err
	}

	var reaped []string

	// serviceGone reports whether the referenced service is confirmed absent.
	// It returns false on transient errors to avoid reaping a healthy webhook.
	serviceGone := func(ns, name string) bool {
		_, gErr := clientset.CoreV1().Services(ns).Get(ctx, name, metav1.GetOptions{})
		return apierrors.IsNotFound(gErr)
	}

	// Validating webhooks
	if vwcs, lErr := clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().List(ctx, metav1.ListOptions{}); lErr == nil {
		for _, vwc := range vwcs.Items {
			for _, wh := range vwc.Webhooks {
				if wh.ClientConfig.Service != nil && serviceGone(wh.ClientConfig.Service.Namespace, wh.ClientConfig.Service.Name) {
					log.Info("Reaping orphaned ValidatingWebhookConfiguration (backing service gone)",
						"name", vwc.Name,
						"service", fmt.Sprintf("%s/%s", wh.ClientConfig.Service.Namespace, wh.ClientConfig.Service.Name))
					if dErr := clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Delete(ctx, vwc.Name, metav1.DeleteOptions{}); dErr == nil {
						reaped = append(reaped, "validating/"+vwc.Name)
					} else {
						log.Warn("Failed to delete orphaned ValidatingWebhookConfiguration", "name", vwc.Name, "err", dErr)
					}
					break
				}
			}
		}
	}

	// Mutating webhooks
	if mwcs, lErr := clientset.AdmissionregistrationV1().MutatingWebhookConfigurations().List(ctx, metav1.ListOptions{}); lErr == nil {
		for _, mwc := range mwcs.Items {
			for _, wh := range mwc.Webhooks {
				if wh.ClientConfig.Service != nil && serviceGone(wh.ClientConfig.Service.Namespace, wh.ClientConfig.Service.Name) {
					log.Info("Reaping orphaned MutatingWebhookConfiguration (backing service gone)",
						"name", mwc.Name,
						"service", fmt.Sprintf("%s/%s", wh.ClientConfig.Service.Namespace, wh.ClientConfig.Service.Name))
					if dErr := clientset.AdmissionregistrationV1().MutatingWebhookConfigurations().Delete(ctx, mwc.Name, metav1.DeleteOptions{}); dErr == nil {
						reaped = append(reaped, "mutating/"+mwc.Name)
					} else {
						log.Warn("Failed to delete orphaned MutatingWebhookConfiguration", "name", mwc.Name, "err", dErr)
					}
					break
				}
			}
		}
	}

	return reaped, nil
}

// HelmReleaseStuckGracePeriod is how long a Helm release record may sit in a
// transient ("uninstalling"/"pending-*") status before ScrubStuckHelmReleaseSecrets
// considers it genuinely wedged rather than an operation a controller is
// actively driving. A healthy install/upgrade completes well within this window
// (especially with Flux's disableWait, which returns as soon as manifests are
// applied), so anything still pending past it has been abandoned by a dead or
// looping controller.
const HelmReleaseStuckGracePeriod = 10 * time.Minute

// ScrubStuckHelmReleaseSecrets deletes Helm v3 release-record Secrets that are
// stuck in a non-terminal status (uninstalling, pending-install,
// pending-upgrade, pending-rollback). Helm stores one Secret per release
// revision (type "helm.sh/release.v1") and refuses to start a new operation
// while the latest revision is in one of these transient states, failing with
// "another operation (install/upgrade/rollback) is in progress". This happens
// when a destroy is interrupted or a controller dies mid-uninstall, and it
// deadlocks any subsequent re-install of that release.
//
// Only transient-status records are removed; "deployed", "failed", and
// "superseded" revisions are left intact so release history and rollback
// targets are preserved. Returns the namespace/name of each scrubbed secret.
//
// minAge guards against orphaning a release that a controller is *actively*
// reconciling: a transient record younger than minAge is left alone, because
// helm-controller creates the pending revision secret at the start of an
// operation and expects to find it when finishing. Deleting that secret out
// from under an in-flight upgrade leaves the controller looping forever on
// "secrets sh.helm.release.v1.<name>.v<N> not found". Pass 0 to scrub
// unconditionally (safe during teardown, when Flux is already suspended and no
// reconciliation is in progress).
func (c KubernetesClient) ScrubStuckHelmReleaseSecrets(ctx context.Context, minAge time.Duration) ([]string, error) {
	clientset, err := c.api.ClientSet()
	if err != nil {
		return nil, err
	}

	stuckStatuses := map[string]bool{
		"uninstalling":     true,
		"pending-install":  true,
		"pending-upgrade":  true,
		"pending-rollback": true,
	}

	// Helm release records are Secrets labelled owner=helm; the per-revision
	// status is exposed as the "status" label so we can filter server-side.
	secrets, err := clientset.CoreV1().Secrets("").List(ctx, metav1.ListOptions{
		LabelSelector: "owner=helm",
	})
	if err != nil {
		return nil, err
	}

	now := time.Now()
	var scrubbed []string
	for _, s := range secrets.Items {
		if s.Type != "helm.sh/release.v1" {
			continue
		}
		if !stuckStatuses[s.Labels["status"]] {
			continue
		}

		// Skip records still within the grace window — these belong to an
		// operation a controller is most likely actively driving. Deleting an
		// in-flight revision secret is unrecoverable for helm-controller, which
		// expects to find it when it completes the operation.
		if minAge > 0 {
			if age := now.Sub(s.CreationTimestamp.Time); age < minAge {
				log.Debug("Skipping recently-created Helm release secret (likely active reconcile)",
					"namespace", s.Namespace,
					"name", s.Name,
					"release", s.Labels["name"],
					"status", s.Labels["status"],
					"age", age.Round(time.Second))
				continue
			}
		}

		log.Info("Scrubbing stuck Helm release secret",
			"namespace", s.Namespace,
			"name", s.Name,
			"release", s.Labels["name"],
			"status", s.Labels["status"])

		if dErr := clientset.CoreV1().Secrets(s.Namespace).Delete(ctx, s.Name, metav1.DeleteOptions{}); dErr != nil {
			log.Warn("Failed to delete stuck Helm release secret", "namespace", s.Namespace, "name", s.Name, "err", dErr)
			continue
		}
		scrubbed = append(scrubbed, fmt.Sprintf("%s/%s", s.Namespace, s.Name))
	}

	return scrubbed, nil
}

// AssessExternalSecretsTeardown determines whether a timed-out
// external-secrets Helm uninstall is only a stale bookkeeping problem or
// whether real release resources still remain. Missing CRDs are treated as
// already-cleaned-up, which is the expected steady state late in teardown.
func (c KubernetesClient) AssessExternalSecretsTeardown(ctx context.Context) (ExternalSecretsTeardownAssessment, error) {
	const namespace = "external-secrets"

	assessment := ExternalSecretsTeardownAssessment{}

	clientset, err := c.api.ClientSet()
	if err != nil {
		return assessment, err
	}

	secrets, err := clientset.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "owner=helm",
	})
	if err != nil && !apierrors.IsNotFound(err) {
		return assessment, err
	}
	if err == nil {
		for _, s := range secrets.Items {
			if s.Type != "helm.sh/release.v1" {
				continue
			}
			if release := s.Labels["name"]; release != "" && release != "external-secrets" {
				continue
			}
			assessment.HelmReleaseSecrets = append(assessment.HelmReleaseSecrets, fmt.Sprintf("%s/%s", s.Namespace, s.Name))
		}
	}

	type resourceProbe struct {
		kind       string
		namespaced bool
	}

	probes := []resourceProbe{
		{kind: "ExternalSecret", namespaced: true},
		{kind: "SecretStore", namespaced: true},
		{kind: "PushSecret", namespaced: true},
		{kind: "ClusterSecretStore", namespaced: false},
		{kind: "ClusterPushSecret", namespaced: false},
		{kind: "ClusterExternalSecret", namespaced: false},
	}

	for _, probe := range probes {
		gvr, lookupErr := c.LookupKind(ctx, probe.kind)
		if lookupErr != nil {
			continue
		}

		scope := ""
		if probe.namespaced {
			scope = namespace
		}

		listErr := c.ForEachDynamicResources(ctx, gvr, scope, func(item unstructured.Unstructured) {
			name := item.GetName()
			if ns := item.GetNamespace(); ns != "" {
				assessment.RemainingResources = append(assessment.RemainingResources, fmt.Sprintf("%s %s/%s", probe.kind, ns, name))
				return
			}
			assessment.RemainingResources = append(assessment.RemainingResources, fmt.Sprintf("%s %s", probe.kind, name))
		})
		if listErr != nil && !apierrors.IsNotFound(listErr) {
			return assessment, listErr
		}
	}

	switch {
	case len(assessment.HelmReleaseSecrets) == 0 && len(assessment.RemainingResources) == 0:
		assessment.Recoverable = true
		assessment.Summary = "no Helm release secrets or External Secrets resources remain"
	case len(assessment.HelmReleaseSecrets) > 0 && len(assessment.RemainingResources) == 0:
		assessment.Summary = fmt.Sprintf("%d Helm release secret(s) remain", len(assessment.HelmReleaseSecrets))
	case len(assessment.HelmReleaseSecrets) == 0 && len(assessment.RemainingResources) > 0:
		assessment.Summary = fmt.Sprintf("%d External Secrets resource(s) remain", len(assessment.RemainingResources))
	default:
		assessment.Summary = fmt.Sprintf("%d Helm release secret(s) and %d External Secrets resource(s) remain", len(assessment.HelmReleaseSecrets), len(assessment.RemainingResources))
	}

	return assessment, nil
}

type PodHealthStatus struct {
	TotalPods    int
	ReadyPods    int
	CrashLooping []string // Pod names in CrashLoopBackOff
	ImagePullErr []string // Pod names with ImagePullBackOff/ErrImagePull
	InitErrors   []string // Pod names stuck in Init
	PendingPods  []string // Pod names in Pending state
}

// IsHealthy returns true if no pods are in a terminal error state.
func (s PodHealthStatus) IsHealthy() bool {
	return len(s.CrashLooping) == 0 && len(s.ImagePullErr) == 0 && len(s.InitErrors) == 0
}

// CheckPodHealth checks the health of pods matching the given app label in a namespace.
// If namespace is empty, checks all namespaces. Returns a PodHealthStatus summary.
func (c KubernetesClient) CheckPodHealth(ctx context.Context, namespace string, appLabel string) (PodHealthStatus, error) {
	clientset, err := c.api.ClientSet()
	if err != nil {
		return PodHealthStatus{}, err
	}

	listOpts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app.kubernetes.io/name=%s", appLabel),
	}

	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, listOpts)
	if err != nil {
		return PodHealthStatus{}, err
	}

	status := PodHealthStatus{TotalPods: len(pods.Items)}
	for _, pod := range pods.Items {
		podName := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)

		if pod.Status.Phase == corev1.PodRunning {
			// Check container statuses for crash loops
			for _, cs := range pod.Status.ContainerStatuses {
				if cs.Ready {
					status.ReadyPods++
					continue
				}
				if cs.State.Waiting != nil {
					switch cs.State.Waiting.Reason {
					case "CrashLoopBackOff":
						status.CrashLooping = append(status.CrashLooping, podName)
					case "ImagePullBackOff", "ErrImagePull":
						status.ImagePullErr = append(status.ImagePullErr, podName)
					}
				}
			}
		} else if pod.Status.Phase == corev1.PodPending {
			status.PendingPods = append(status.PendingPods, podName)
			// Check init container statuses
			for _, cs := range pod.Status.InitContainerStatuses {
				if cs.State.Waiting != nil && cs.State.Waiting.Reason == "CrashLoopBackOff" {
					status.InitErrors = append(status.InitErrors, podName)
				}
			}
		}
	}

	return status, nil
}

// ForEachDynamicResources iterates over all dynamic resources of a specific kind and namespace.
func (c KubernetesClient) ForEachDynamicResources(ctx context.Context, kind schema.GroupVersionResource, ns string, onEachItem func(unstructured.Unstructured)) error {
	dyn, err := c.api.DynamicClient()
	if err != nil {
		return err
	}

	i := dyn.Resource(kind)
	if ns == "" {
		l, err := i.List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}

		for _, li := range l.Items {
			onEachItem(li)
		}

		return nil
	}

	l, err := i.Namespace(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, li := range l.Items {
		onEachItem(li)
	}

	return nil
}

// Update updates a dynamic resource in the cluster.
func (c KubernetesClient) Update(ctx context.Context, kind schema.GroupVersionResource, ns string, obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	dyn, err := c.api.DynamicClient()
	if err != nil {
		return obj, err
	}

	i := dyn.Resource(kind)
	if ns == "" {
		u, err := i.Update(ctx, obj, metav1.UpdateOptions{})
		if err != nil {
			return u, err
		}

		return u, nil
	}

	u, err := i.Namespace(ns).Update(ctx, obj, metav1.UpdateOptions{})
	if err != nil {
		return u, err
	}

	return u, nil
}

// GetDaemonSetStatus retrieves the ready and desired replica counts for a DaemonSet.
// This is used to verify that all DaemonSet pods are running on all applicable nodes,
// which is critical for CNI plugins like istio-cni that must be fully deployed
// before other pods can be scheduled.
func (c KubernetesClient) GetDaemonSetStatus(ctx context.Context, kind schema.GroupVersionResource, ns string, name string) (int64, int64, error) {
	obj, err := c.GetDynamicResource(ctx, kind, ns, name)
	if err != nil {
		return 0, 0, err
	}

	desired, found, err := unstructured.NestedInt64(obj, "status", "desiredNumberScheduled")
	if err != nil || !found {
		return 0, 0, fmt.Errorf("could not get desiredNumberScheduled for DaemonSet %s/%s: %v", ns, name, err)
	}

	ready, _, _ := unstructured.NestedInt64(obj, "status", "numberReady")

	return ready, desired, nil
}

// Restart restarts resources of a specific kind in the cluster.
func (c KubernetesClient) Restart(ctx context.Context, kind schema.GroupVersionResource, ns string, name string) error {
	validRes := []string{"Deployments", "DaemonSets", "StatefulSets"}
	if !slices.ContainsFunc(validRes, func(s string) bool {
		return strings.EqualFold(s, kind.Resource)
	}) {
		return fmt.Errorf("unsupported resource type %s, must be one of %v", kind.Resource, validRes)
	}

	// for each item in result, update spec/template/metadata/annoations to trigger rollout
	timestamp := time.Now().UTC().Format(time.RFC3339)
	return c.ForEachDynamicResources(ctx, kind, ns, func(item unstructured.Unstructured) {
		n := item.GetName()
		ns := item.GetNamespace()

		if name != "" && !strings.EqualFold(n, name) {
			// TODO: quick fix, make this more efficient
			log.Debug("skipping due to name mismatch", "resource", kind.Resource, "namespace", ns, "name", name)
			return
		}

		util.Printf("Triggering refresh of %s %s/%s", kind.Resource, ns, n)

		annotations, found, err := unstructured.NestedStringMap(item.Object, "spec", "template", "metadata", "annotations")
		if err != nil || !found || annotations == nil {
			annotations = map[string]string{}
		}

		// https://stackoverflow.com/questions/61335318/how-to-restart-a-deployment-in-kubernetes-using-go-client
		annotations["kubectl.kubernetes.io/restartedAt"] = timestamp
		err = unstructured.SetNestedStringMap(item.Object, annotations, "spec", "template", "metadata", "annotations")
		if err != nil {
			log.Error("failed to set restart annotation", "err", err)
			return
		}

		_, ierr := c.Update(ctx, kind, ns, &item)
		if ierr != nil {
			log.Info("Error updating dynamic resource", "kind", kind, "name", n, "ns", ns, "err", ierr)
		}
	})
}

// requestServiceAccountToken requests a token for a service account.
func requestServiceAccountToken(ctx context.Context, cfg quartzSchema.QuartzConfig, rc *rest.Config) (string, error) {
	clientset, err := kubernetes.NewForConfig(rc)
	if err != nil {
		return "", err
	}

	saClient := clientset.CoreV1().ServiceAccounts(cfg.Auth.ServiceAccount.Namespace)

	tr, err := saClient.CreateToken(ctx, cfg.Auth.ServiceAccount.Name, &authv1.TokenRequest{
		Spec: authv1.TokenRequestSpec{
			Audiences:         []string{},
			ExpirationSeconds: &cfg.Auth.ServiceAccount.ExpirationSeconds,
		},
	}, metav1.CreateOptions{})

	if err != nil {
		return "", err
	}

	return tr.Status.Token, nil
}

// ToKubeconfigYamlBytes converts the KubeconfigInfo to YAML bytes.
func (kc KubeconfigInfo) ToKubeconfigYamlBytes(cfg quartzSchema.QuartzConfig) []byte {
	kubeconfig := kc.Kubeconfig(cfg)
	return util.MarshalToYamlBytes(kubeconfig)
}

// Kubeconfig converts the KubeconfigInfo to a Kubeconfig structure.
func (kc KubeconfigInfo) Kubeconfig(cfg quartzSchema.QuartzConfig) quartzSchema.Kubeconfig {
	var user quartzSchema.KubeconfigUserInfo
	if !cfg.Auth.ServiceAccount.Enabled && cfg.Providers.Cloud == "aws" {
		bin, _ := os.Executable()
		user = quartzSchema.KubeconfigUserInfo{
			Exec: &quartzSchema.KubeconfigUserExec{
				ApiVersion: "client.authentication.k8s.io/v1beta1",
				Command:    bin,
				Args: []string{
					"aws",
					"get-eks-token",
					"--cluster",
					cfg.Name,
					"--region",
					cfg.Aws.Region,
				},
			},
		}
	} else {
		user = quartzSchema.KubeconfigUserInfo{
			Token: &kc.Token,
		}
	}

	return quartzSchema.Kubeconfig{
		ApiVersion:     "v1",
		Kind:           "Config",
		CurrentContext: kc.Context,
		Clusters: []quartzSchema.KubeconfigCluster{
			{
				Name: kc.Cluster,
				Cluster: quartzSchema.KubeconfigClusterInfo{
					Server:                   kc.Endpoint,
					CertificateAuthorityData: kc.CertificateAuthority,
				},
			},
		},
		Contexts: []quartzSchema.KubeconfigContext{
			{
				Name: kc.Context,
				Context: quartzSchema.KubeconfigContextInfo{
					Cluster: kc.Cluster,
					User:    kc.User,
				},
			},
		},
		Users: []quartzSchema.KubeconfigUser{
			{
				Name: kc.User,
				User: user,
			},
		},
	}
}

// ToTable converts the KubernetesProviderCheckResult into table headers and rows for display.
func (r KubernetesProviderCheckResult) ToTable() ([]string, []ProviderCheckResultRow) {
	headers := []string{"Cluster"}
	rows := []ProviderCheckResultRow{
		{
			Status: r.Status,
			Error:  r.Error,
			Data:   []string{r.cfg.Name},
		},
	}

	return headers, rows
}

// PrepareForDestroy monitors the cluster before infrastructure destruction.
// The actual cleanup is handled by Helm pre-delete hooks in the base and
// karpenter_resources charts. This function verifies that the cluster is in
// a state ready for destruction and logs any cleanup that appears incomplete.
//
// The cleanup phases (defined in Helm hooks):
//  1. Delete LoadBalancer Services → LB controller removes NLBs and their SGs
//  2. Scale Flux controllers to 0 → prevents re-reconciliation during cleanup
//  3. Strip finalizers from Flux CRs → unblocks CR deletion
//  4. Delete all Flux CRs → removes objects that would block CRD/namespace deletion
//  5. Drain Karpenter nodes → terminates Karpenter-managed EC2 instances
//  6. Evict all pods from managed nodes → triggers CNI DEL for each pod → ENI release
//  7. Delete aws-node DaemonSet → graceful CNI shutdown releases trunk/branch ENIs
//  8. Patch stuck namespaces → removes finalizers on Terminating namespaces
//
// If cleanup via hooks is incomplete, this function will explicitly handle
// the cleanup to ensure infrastructure destruction can proceed.
func (c KubernetesClient) PrepareForDestroy(ctx context.Context) error {
	log.Debug("Entering", "internal", "prepareForDestroy")
	defer log.Debug("Completed", "internal", "prepareForDestroy")

	clientset, err := c.api.ClientSet()
	if err != nil {
		return fmt.Errorf("failed to get clientset: %w", err)
	}

	// Verify that cleanup is progressing. If critical resources still exist,
	// log it as a warning but don't fail — the hooks may still be running
	// or cleanup may have been partially completed.

	// Check for LoadBalancer services
	services, err := clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err == nil && services != nil {
		var lbServices []string
		for _, svc := range services.Items {
			if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
				lbServices = append(lbServices, fmt.Sprintf("%s/%s", svc.Namespace, svc.Name))
			}
		}
		if len(lbServices) > 0 {
			log.Warn("LoadBalancer services still exist (cleanup may still be in progress)",
				"services", len(lbServices), "examples", lbServices[:1])
		}
	}

	// Check for non-system pods that should have been evicted
	pods, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err == nil && pods != nil {
		var appPods []string
		for _, pod := range pods.Items {
			// Skip system namespaces
			if pod.Namespace == "kube-system" || pod.Namespace == "karpenter" || pod.Namespace == "kube-node-lease" {
				continue
			}
			appPods = append(appPods, fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))
			if len(appPods) >= 3 {
				break
			}
		}
		if len(appPods) > 0 {
			log.Warn("Non-system pods still exist (cleanup may still be in progress)",
				"pods", len(appPods), "examples", appPods)
		}
	}

	// Check for stuck Terminating namespaces and patch them
	namespaces, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err == nil && namespaces != nil {
		for _, ns := range namespaces.Items {
			if ns.Status.Phase != corev1.NamespaceTerminating {
				continue
			}
			if len(ns.Spec.Finalizers) == 0 {
				continue
			}
			// Namespace is stuck in Terminating — remove spec finalizers
			ns.Spec.Finalizers = nil
			_, err := clientset.CoreV1().Namespaces().Finalize(ctx, &ns, metav1.UpdateOptions{})
			if err != nil {
				log.Warn("Failed to patch stuck namespace", "namespace", ns.Name, "err", err)
				continue
			}
			log.Info("Patched stuck namespace", "namespace", ns.Name)
		}
	}

	log.Info("PrepareForDestroy verification complete")
	return nil
}

// InterStageCleanup performs lightweight K8s cleanup between stage destroys.
// It catches issues that emerge after a stage is destroyed (e.g., orphaned webhooks,
// stuck finalizers on services whose controller was just destroyed).
func (c KubernetesClient) InterStageCleanup(ctx context.Context) error {
	log.Debug("Entering", "internal", "interStageCleanup")
	defer log.Debug("Completed", "internal", "interStageCleanup")

	clientset, err := c.api.ClientSet()
	if err != nil {
		return fmt.Errorf("failed to get clientset: %w", err)
	}

	// 1. Delete orphaned Validating/MutatingWebhookConfigurations whose
	// backing service is gone (shared with the install-time safety net).
	if _, err := c.ReapOrphanedAdmissionWebhooks(ctx); err != nil {
		log.Debug("Orphaned webhook reaping during inter-stage cleanup failed (non-fatal)", "err", err)
	}

	// 2. Delete Helm release secrets stuck in a transient state (uninstalling/
	// pending-*). When a stage destroy is interrupted or a controller dies
	// mid-uninstall, Helm leaves its release record secret in a non-terminal
	// status; the next operation then aborts with "another operation is in
	// progress", deadlocking re-install of that release. Scrubbing only the
	// stuck records lets Helm recover without touching healthy deployments.
	// Flux is already suspended during teardown, so there is no active
	// reconciliation to protect — scrub unconditionally (minAge 0).
	if _, err := c.ScrubStuckHelmReleaseSecrets(ctx, 0); err != nil {
		log.Debug("Stuck helm release secret scrub during inter-stage cleanup failed (non-fatal)", "err", err)
	}

	// 3. Patch finalizers on LoadBalancer services whose controller is dead
	services, err := clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, svc := range services.Items {
			if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
				continue
			}
			if len(svc.Finalizers) == 0 {
				continue
			}
			// Check if LB controller is running
			controllerPods, _ := clientset.CoreV1().Pods("kube-system").List(ctx, metav1.ListOptions{
				LabelSelector: "app.kubernetes.io/name=aws-load-balancer-controller",
			})
			controllerAlive := false
			if controllerPods != nil {
				for _, p := range controllerPods.Items {
					if p.Status.Phase == corev1.PodRunning {
						controllerAlive = true
						break
					}
				}
			}
			if !controllerAlive {
				svc.Finalizers = nil
				_, patchErr := clientset.CoreV1().Services(svc.Namespace).Update(ctx, &svc, metav1.UpdateOptions{})
				if patchErr == nil {
					log.Info("Stripped finalizers from LB service (controller dead)", "service", svc.Namespace+"/"+svc.Name)
				}
			}
		}
	}

	// 4. Force-delete pods stuck in Terminating
	pods, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err == nil {
		zero := int64(0)
		for _, pod := range pods.Items {
			if pod.Namespace == "kube-system" {
				continue
			}
			if pod.DeletionTimestamp != nil {
				_ = clientset.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{
					GracePeriodSeconds: &zero,
				})
			}
		}
	}

	// 5. Patch stuck Terminating namespaces
	namespaces, err := clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, ns := range namespaces.Items {
			if ns.Status.Phase != corev1.NamespaceTerminating {
				continue
			}
			if len(ns.Spec.Finalizers) == 0 {
				continue
			}
			ns.Spec.Finalizers = nil
			_, patchErr := clientset.CoreV1().Namespaces().Finalize(ctx, &ns, metav1.UpdateOptions{})
			if patchErr == nil {
				log.Info("Patched stuck namespace during inter-stage cleanup", "namespace", ns.Name)
			}
		}
	}

	return nil
}
