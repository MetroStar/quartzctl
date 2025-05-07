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
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"
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
	}
	c.PrintClusterAppInfo(ctx, apps)
}

// PrintClusterAppInfo prints detailed information about the specified applications in the cluster.
func (c KubernetesClient) PrintClusterAppInfo(ctx context.Context, apps map[string]quartzSchema.ApplicationLookupConfig) {
	ch := make(chan KubernetesAppConnectionInfo, len(apps))

	for k, v := range apps {
		app := k
		opts := v
		go func() {
			ch <- c.GetAppConnectionInfo(ctx, app, opts)
		}()
	}

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
	close(ch)

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

	return res, kerrors.NewAggregate(errs)
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
		res.Error = kerrors.NewAggregate(errs)
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
