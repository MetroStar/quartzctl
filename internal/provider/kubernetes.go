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
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	quartzSchema "github.com/MetroStar/quartzctl/internal/config/schema"
	"github.com/MetroStar/quartzctl/internal/log"
	"github.com/MetroStar/quartzctl/internal/util"

	authv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"kmodules.xyz/client-go/tools/wait"
	"sigs.k8s.io/yaml"
)

var defaultCache = &KubernetesLookupCache{
	mutex: &sync.Mutex{},
	kinds: map[string]schema.GroupVersionResource{},
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
	GetSecretValue(ctx context.Context, ns string, name string) (map[string]string, error)
	Restart(ctx context.Context, kind schema.GroupVersionResource, ns string, name string) error
	GetDaemonSetStatus(ctx context.Context, kind schema.GroupVersionResource, ns string, name string) (int64, int64, error)
	CleanupStuckTerminatingPods(ctx context.Context, timeout time.Duration) ([]string, error)
	ListVirtualServices(ctx context.Context) ([]VirtualServiceInfo, error)
	PrepareForDestroy(ctx context.Context) error
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

// KubernetesAppConnectionInfo contains information about an application's connection in Kubernetes.
type KubernetesAppConnectionInfo struct {
	Name           string
	PublicEndpoint string
	AdminUsername  string
	AdminPassword  string
	Error          error
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

	util.PrintTable(headers, rows)
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

// PodHealthStatus represents the health state of pods matching a selector.
type PodHealthStatus struct {
	TotalPods     int
	ReadyPods     int
	CrashLooping  []string // Pod names in CrashLoopBackOff
	ImagePullErr  []string // Pod names with ImagePullBackOff/ErrImagePull
	InitErrors    []string // Pod names stuck in Init
	PendingPods   []string // Pod names in Pending state
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

// scaleFluxControllers scales all Flux controller Deployments in flux-system
// to the specified number of replicas.
func (c KubernetesClient) scaleFluxControllers(ctx context.Context, clientset kubernetes.Interface, replicas int32) {
	const ns = "flux-system"
	deployments, err := clientset.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Debug("Could not list Flux deployments (skipping)", "err", err)
		return
	}

	var scaled int
	for _, d := range deployments.Items {
		if d.Spec.Replicas != nil && *d.Spec.Replicas == replicas {
			continue
		}
		scale, err := clientset.AppsV1().Deployments(ns).GetScale(ctx, d.Name, metav1.GetOptions{})
		if err != nil {
			log.Warn("Failed to get scale for deployment", "name", d.Name, "err", err)
			continue
		}
		scale.Spec.Replicas = replicas
		_, err = clientset.AppsV1().Deployments(ns).UpdateScale(ctx, d.Name, scale, metav1.UpdateOptions{})
		if err != nil {
			log.Warn("Failed to scale deployment", "name", d.Name, "err", err)
			continue
		}
		scaled++
	}

	if scaled > 0 {
		util.Printf("Scaled %d Flux controller(s) to 0 replicas", scaled)
		// Brief wait for controller pods to terminate
		time.Sleep(10 * time.Second)
	}
}

// cleanupFluxResources strips finalizers from all Flux CRs and deletes them.
func (c KubernetesClient) cleanupFluxResources(ctx context.Context, dyn dynamic.Interface) {
	fluxCRDs := []schema.GroupVersionResource{
		{Group: "helm.toolkit.fluxcd.io", Version: "v2", Resource: "helmreleases"},
		{Group: "source.toolkit.fluxcd.io", Version: "v1", Resource: "gitrepositories"},
		{Group: "source.toolkit.fluxcd.io", Version: "v1", Resource: "helmrepositories"},
		{Group: "source.toolkit.fluxcd.io", Version: "v1", Resource: "helmcharts"},
		{Group: "kustomize.toolkit.fluxcd.io", Version: "v1", Resource: "kustomizations"},
		{Group: "notification.toolkit.fluxcd.io", Version: "v1beta3", Resource: "alerts"},
		{Group: "notification.toolkit.fluxcd.io", Version: "v1", Resource: "receivers"},
		{Group: "notification.toolkit.fluxcd.io", Version: "v1beta3", Resource: "providers"},
	}

	var totalCleaned int
	for _, gvr := range fluxCRDs {
		list, err := dyn.Resource(gvr).Namespace("").List(ctx, metav1.ListOptions{})
		if err != nil {
			log.Debug("Skipping CRD (not found or inaccessible)", "resource", gvr.Resource, "group", gvr.Group)
			continue
		}

		for _, item := range list.Items {
			// Strip finalizers if present
			if len(item.GetFinalizers()) > 0 {
				item.SetFinalizers(nil)
				_, err := dyn.Resource(gvr).Namespace(item.GetNamespace()).Update(ctx, &item, metav1.UpdateOptions{})
				if err != nil {
					log.Warn("Failed to remove finalizers", "resource", gvr.Resource, "namespace", item.GetNamespace(), "name", item.GetName(), "err", err)
					continue
				}
				totalCleaned++
			}

			// Delete the CR so it doesn't block CRD or namespace deletion
			err := dyn.Resource(gvr).Namespace(item.GetNamespace()).Delete(ctx, item.GetName(), metav1.DeleteOptions{})
			if err != nil {
				log.Debug("Failed to delete Flux CR (may already be gone)", "resource", gvr.Resource, "name", item.GetName(), "err", err)
			}
		}
	}

	if totalCleaned > 0 {
		util.Printf("Cleaned up %d Flux resource(s) (finalizers removed + deleted)", totalCleaned)
	}
}

// evictAllPods cordons all nodes and evicts non-system pods. Each eviction triggers
// the CNI plugin's DEL callback, which releases the pod's ENI allocation naturally.
func (c KubernetesClient) evictAllPods(ctx context.Context, clientset kubernetes.Interface) error {
	nodes, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}

	if len(nodes.Items) == 0 {
		log.Debug("No nodes found (skipping pod eviction)")
		return nil
	}

	// Cordon all nodes to prevent rescheduling
	for i := range nodes.Items {
		if nodes.Items[i].Spec.Unschedulable {
			continue
		}
		nodes.Items[i].Spec.Unschedulable = true
		_, err := clientset.CoreV1().Nodes().Update(ctx, &nodes.Items[i], metav1.UpdateOptions{})
		if err != nil {
			log.Warn("Failed to cordon node", "node", nodes.Items[i].Name, "err", err)
		}
	}

	// Collect non-system pods to evict
	pods, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	systemNamespaces := map[string]bool{
		"kube-system": true,
		"kube-node-lease": true,
		"kube-public": true,
	}

	var toEvict []corev1.Pod
	for _, pod := range pods.Items {
		if systemNamespaces[pod.Namespace] {
			continue
		}
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}
		// Skip mirror pods (managed by kubelet directly)
		if _, isMirror := pod.Annotations[corev1.MirrorPodAnnotationKey]; isMirror {
			continue
		}
		toEvict = append(toEvict, pod)
	}

	if len(toEvict) == 0 {
		log.Debug("No pods to evict")
		return nil
	}

	util.Printf("Evicting %d pod(s) to trigger CNI cleanup...", len(toEvict))

	gracePeriod := int64(30)
	for _, pod := range toEvict {
		err := clientset.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{
			GracePeriodSeconds: &gracePeriod,
		})
		if err != nil {
			log.Debug("Failed to evict pod (may already be gone)", "namespace", pod.Namespace, "name", pod.Name, "err", err)
		}
	}

	// Wait for pods to terminate (CNI DEL fires during termination)
	util.Printf("Waiting for pod termination and CNI cleanup...")
	timeout := 2 * time.Minute
	poll := 5 * time.Second
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		remaining, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
		if err != nil {
			break
		}
		var active int
		for _, pod := range remaining.Items {
			if systemNamespaces[pod.Namespace] {
				continue
			}
			if pod.Status.Phase == corev1.PodRunning || pod.Status.Phase == corev1.PodPending {
				active++
			}
		}
		if active == 0 {
			util.Printf("All non-system pods terminated")
			return nil
		}
		log.Debug("Waiting for pod termination", "remaining", active)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(poll):
		}
	}

	log.Warn("Timed out waiting for pod termination (continuing)")
	return nil
}

// deleteCNIDaemonSet deletes the aws-node DaemonSet in kube-system. When the VPC
// CNI pods terminate gracefully (while nodes are still alive), they release trunk
// and branch ENIs that were allocated for pod networking on each node.
func (c KubernetesClient) deleteCNIDaemonSet(ctx context.Context, clientset kubernetes.Interface) error {
	const (
		ns   = "kube-system"
		name = "aws-node"
	)

	_, err := clientset.AppsV1().DaemonSets(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		log.Debug("aws-node DaemonSet not found (skipping)", "err", err)
		return nil
	}

	util.Printf("Deleting %s/%s DaemonSet for graceful ENI release...", ns, name)
	err = clientset.AppsV1().DaemonSets(ns).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete aws-node DaemonSet: %w", err)
	}

	// Wait for CNI pods to terminate — this is when ENIs are actually released
	timeout := 90 * time.Second
	poll := 5 * time.Second
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		pods, err := clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{
			LabelSelector: "k8s-app=aws-node",
		})
		if err != nil || len(pods.Items) == 0 {
			util.Printf("VPC CNI pods terminated — ENIs released")
			return nil
		}
		log.Debug("Waiting for CNI pod termination", "remaining", len(pods.Items))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(poll):
		}
	}

	log.Warn("Timed out waiting for CNI pod termination (continuing)")
	return nil
}

// deleteLoadBalancerServices deletes all Services of type LoadBalancer and waits
// for the AWS LB controller to reconcile (remove NLBs and security groups).
func (c KubernetesClient) deleteLoadBalancerServices(ctx context.Context, clientset kubernetes.Interface) error {
	services, err := clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list services: %w", err)
	}

	var lbServices []corev1.Service
	for _, svc := range services.Items {
		if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
			lbServices = append(lbServices, svc)
		}
	}

	if len(lbServices) == 0 {
		log.Debug("No LoadBalancer services found")
		return nil
	}

	util.Printf("Deleting %d LoadBalancer service(s) for LB controller cleanup...", len(lbServices))
	for _, svc := range lbServices {
		log.Debug("Deleting LoadBalancer service", "namespace", svc.Namespace, "name", svc.Name)
		err := clientset.CoreV1().Services(svc.Namespace).Delete(ctx, svc.Name, metav1.DeleteOptions{})
		if err != nil {
			log.Warn("Failed to delete LoadBalancer service", "namespace", svc.Namespace, "name", svc.Name, "err", err)
			continue
		}
		util.Printf("  Deleted %s/%s", svc.Namespace, svc.Name)
	}

	// Wait for services to be fully removed (finalizer cleared by LB controller = SGs cleaned up)
	util.Printf("Waiting for LB controller to clean up cloud resources...")
	timeout := 5 * time.Minute
	poll := 5 * time.Second
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		allGone := true
		for _, svc := range lbServices {
			_, err := clientset.CoreV1().Services(svc.Namespace).Get(ctx, svc.Name, metav1.GetOptions{})
			if err == nil {
				allGone = false
				break
			}
		}
		if allGone {
			util.Printf("All LoadBalancer services cleaned up successfully")
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(poll):
		}
	}

	log.Warn("Timed out waiting for LoadBalancer service cleanup (continuing)")
	return nil
}

// drainKarpenterNodes deletes all Karpenter NodePool and NodeClaim resources,
// triggering Karpenter to terminate the underlying EC2 instances. This prevents
// orphaned ENIs from blocking VPC subnet and security group deletion during destroy.
func (c KubernetesClient) drainKarpenterNodes(ctx context.Context, dyn dynamic.Interface) error {
	nodeClaimGVR := schema.GroupVersionResource{
		Group: "karpenter.sh", Version: "v1", Resource: "nodeclaims",
	}
	nodePoolGVR := schema.GroupVersionResource{
		Group: "karpenter.sh", Version: "v1", Resource: "nodepools",
	}

	// Delete NodePools first to prevent Karpenter from replacing terminated nodes
	pools, err := dyn.Resource(nodePoolGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		log.Debug("Karpenter NodePool CRD not found (skipping)", "err", err)
		return nil
	}

	for _, pool := range pools.Items {
		log.Debug("Deleting NodePool", "name", pool.GetName())
		err := dyn.Resource(nodePoolGVR).Delete(ctx, pool.GetName(), metav1.DeleteOptions{})
		if err != nil {
			log.Warn("Failed to delete NodePool", "name", pool.GetName(), "err", err)
		}
	}

	if len(pools.Items) > 0 {
		util.Printf("Deleted %d Karpenter NodePool(s)", len(pools.Items))
	}

	// Delete all NodeClaims to trigger instance termination
	claims, err := dyn.Resource(nodeClaimGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil
	}

	if len(claims.Items) == 0 {
		log.Debug("No Karpenter NodeClaims found")
		return nil
	}

	util.Printf("Draining %d Karpenter-managed node(s)...", len(claims.Items))
	for _, claim := range claims.Items {
		log.Debug("Deleting NodeClaim", "name", claim.GetName())
		err := dyn.Resource(nodeClaimGVR).Delete(ctx, claim.GetName(), metav1.DeleteOptions{})
		if err != nil {
			log.Warn("Failed to delete NodeClaim", "name", claim.GetName(), "err", err)
		}
	}

	// Wait for NodeClaims to be fully removed (instances terminated)
	timeout := 5 * time.Minute
	poll := 10 * time.Second
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		remaining, err := dyn.Resource(nodeClaimGVR).List(ctx, metav1.ListOptions{})
		if err != nil || len(remaining.Items) == 0 {
			util.Printf("All Karpenter nodes terminated successfully")
			return nil
		}
		log.Debug("Waiting for Karpenter nodes to terminate", "remaining", len(remaining.Items))
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(poll):
		}
	}

	log.Warn("Timed out waiting for Karpenter node termination (continuing)")
	return nil
}
