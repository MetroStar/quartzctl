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

// https://github.com/kubernetes/client-go/blob/master/dynamic/fake/simple_test.go

import (
	"bytes"
	"context"
	"encoding/base64"
	"os"
	"path"
	"strings"
	"testing"

	"github.com/MetroStar/quartzctl/internal/config/schema"
	"github.com/MetroStar/quartzctl/internal/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sSchema "k8s.io/apimachinery/pkg/runtime/schema"
)

func TestProviderKubernetesClientCtor(t *testing.T) {
	t.Setenv("KUBECONFIG", "")
	cfg := schema.QuartzConfig{
		Name:      "testcluster",
		Providers: schema.NewProvidersConfig(),
		Auth:      schema.DefaultAuthConfig(),
		Aws:       schema.AwsConfig{Region: "test-region"},
	}
	i := &KubeconfigInfo{
		Context:              "mytestcontext",
		Cluster:              "testcluster",
		User:                 "testuser",
		Endpoint:             "http://nowhere.example.com",
		CertificateAuthority: base64.StdEncoding.EncodeToString([]byte("mytestcert")),
	}
	api, err := NewKubernetesApi(context.Background(), cfg, i)
	if err != nil {
		t.Errorf("unexpected error from real kubernetes client constructor, %v", err)
		return
	}

	clientset, err := api.ClientSet()
	if err != nil {
		t.Errorf("unexpected error from real kubernetes client constructor, %v", err)
	}
	t.Logf("clientset - %v", clientset)

	dynamic, err := api.DynamicClient()
	if err != nil {
		t.Errorf("unexpected error from real kubernetes client constructor, %v", err)
	}
	t.Logf("dynamic - %v", dynamic)

	discovery, err := api.DiscoveryClient()
	if err != nil {
		t.Errorf("unexpected error from real kubernetes client constructor, %v", err)
	}
	t.Logf("discovery - %v", discovery)
}

func TestProviderKubernetesClientServiceAccountAuthEnabled(t *testing.T) {
	t.Setenv("KUBECONFIG", "")
	cfg := schema.QuartzConfig{
		Name:      "testcluster",
		Providers: schema.NewProvidersConfig(),
		Auth:      schema.DefaultAuthConfig(),
		Aws:       schema.AwsConfig{Region: "test-region"},
	}
	i := &KubeconfigInfo{
		Context:              "mytestcontext",
		Cluster:              "testcluster",
		User:                 "testuser",
		Endpoint:             "http://nowhere.example.com",
		CertificateAuthority: base64.StdEncoding.EncodeToString([]byte("mytestcert")),
	}

	cfg.Auth.ServiceAccount.Enabled = true
	api, err := NewKubernetesApi(context.Background(), cfg, i)
	if err == nil {
		t.Errorf("expected error from real kubernetes client constructor, sa token lookup requires live cluster, %v", api)
	}
}

func TestProviderKubernetesClientCtorErr(t *testing.T) {
	t.Setenv("KUBECONFIG", "")
	cfg := schema.QuartzConfig{
		Name:      "testcluster",
		Providers: schema.NewProvidersConfig(),
		Auth:      schema.DefaultAuthConfig(),
		Aws:       schema.AwsConfig{Region: "test-region"},
	}
	i := &KubeconfigInfo{}
	_, err := NewKubernetesApi(context.Background(), cfg, i)
	if err == nil {
		t.Errorf("expected error from real kubernetes client constructor")
		return
	}
}

func TestProviderKubernetesClientProviderName(t *testing.T) {
	api := NewKubernetesApiMock()
	kubeconfig := KubeconfigInfo{}
	cfg := schema.QuartzConfig{}

	c, err := NewKubernetesClient(api, kubeconfig, cfg)
	if err != nil {
		t.Errorf("unexpected error from kubernetes client constructor, %v", err)
	}

	if c.ProviderName() != "Kubernetes" {
		t.Errorf("unexpected default provider name from kubernetes client, expected %v, found %v", "Kubernetes", c.ProviderName())
	}
}

func TestProviderKubernetesClientCheckAccess(t *testing.T) {
	node := &corev1.Node{}
	node.Name = "testnode"
	api := NewKubernetesApiMock().WithClientObjects(node)
	kubeconfig := KubeconfigInfo{}
	cfg := schema.QuartzConfig{}

	c, err := NewKubernetesClient(api, kubeconfig, cfg)
	if err != nil {
		t.Errorf("unexpected error from kubernetes client constructor, %v", err)
		return
	}

	res := c.CheckAccess(context.Background())
	if res == nil {
		t.Error("expected response from kubernetes client check access")
		return
	}

	switch r := res.(type) {
	case KubernetesProviderCheckResult:
		if !r.Status ||
			r.Error != nil {
			t.Errorf("unexpected response value from kubernetes check access, %v", r)
		}
	default:
		t.Errorf("unexpected response type from kubernetes check access, %v", r)
	}
}

func TestProviderKubernetesClientWriteKubeconfig(t *testing.T) {
	api := NewKubernetesApiMock()
	cfg := schema.QuartzConfig{
		Name: "mytestcluster",
		Providers: schema.ProvidersConfig{
			Cloud: "aws",
		},
		Aws: schema.AwsConfig{
			Region: "us-test-1",
		},
		Auth: schema.DefaultAuthConfig(),
	}
	kubeconfig := KubeconfigInfo{}

	c, err := NewKubernetesClient(api, kubeconfig, cfg)
	if err != nil {
		t.Errorf("unexpected error from kubernetes client constructor, %v", err)
	}

	var b bytes.Buffer
	err = c.WriteKubeconfig(&b)
	if err != nil {
		t.Errorf("unexpected error from kubernetes client write kubeconfig, %v", err)
	}

	k := b.String()
	if !strings.Contains(k, "get-eks-token") {
		t.Errorf("unexpected response from kubernetes client write kubeconfig, %v", k)
	}
}

func TestProviderKubernetesClientWriteKubeconfigFile(t *testing.T) {
	api := NewKubernetesApiMock()
	cfg := schema.QuartzConfig{
		Name: "mytestcluster",
		Providers: schema.ProvidersConfig{
			Cloud: "aws",
		},
		Aws: schema.AwsConfig{
			Region: "us-test-1",
		},
		Auth: schema.DefaultAuthConfig(),
	}
	kubeconfig := KubeconfigInfo{}

	c, err := NewKubernetesClient(api, kubeconfig, cfg)
	if err != nil {
		t.Errorf("unexpected error from kubernetes client constructor, %v", err)
	}

	tmp := t.TempDir()
	path := path.Join(tmp, "kubeconfig")
	err = c.WriteKubeconfigFile(path)
	if err != nil {
		t.Errorf("unexpected error from kubernetes client write kubeconfig file, %v", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("unexpected error from kubernetes client write kubeconfig file, %v", err)
	}

	k := string(b)
	if !strings.Contains(k, "get-eks-token") {
		t.Errorf("unexpected response from kubernetes client write kubeconfig file, %v", k)
	}
}

func TestProviderKubernetesClientEnsureKubeconfig(t *testing.T) {
	api := NewKubernetesApiMock()
	cfg := schema.QuartzConfig{
		Name: "mytestcluster",
		Providers: schema.ProvidersConfig{
			Cloud: "aws",
		},
		Aws: schema.AwsConfig{
			Region: "us-test-1",
		},
		Auth: schema.DefaultAuthConfig(),
	}
	kubeconfig := KubeconfigInfo{}

	c, err := NewKubernetesClient(api, kubeconfig, cfg)
	if err != nil {
		t.Errorf("unexpected error from kubernetes client constructor, %v", err)
	}

	tmp := t.TempDir()
	path := path.Join(tmp, "kubeconfig")
	err = c.EnsureKubeconfig(path)
	if err != nil {
		t.Errorf("unexpected error from kubernetes client write kubeconfig file, %v", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("unexpected error from kubernetes client write kubeconfig file, %v", err)
	}

	k := string(b)
	if !strings.Contains(k, "get-eks-token") {
		t.Errorf("unexpected response from kubernetes client write kubeconfig file, %v", k)
	}

	// exists this time
	err = c.EnsureKubeconfig(path)
	if err != nil {
		t.Errorf("unexpected error from kubernetes client write kubeconfig file, %v", err)
	}
}

func TestProviderKubernetesClientForEachDynamicResources(t *testing.T) {
	api := NewKubernetesApiMock().WithDynamicObjects(
		newK8sObject("external-secrets.io/v1beta1", "ExternalSecret", "testns1", "testobj1"),
		newK8sObject("external-secrets.io/v1beta1", "ExternalSecret", "testns1", "testobj2"),
		newK8sObject("external-secrets.io/v1beta1", "ExternalSecret", "testns2", "testobj1"),
	)

	cfg := schema.QuartzConfig{}
	kubeconfig := KubeconfigInfo{}

	c, err := NewKubernetesClient(api, kubeconfig, cfg)
	if err != nil {
		t.Errorf("unexpected error from kubernetes client constructor, %v", err)
		return
	}

	count := 0
	kind := k8sSchema.GroupVersionResource{
		Group:    "external-secrets.io",
		Version:  "v1beta1",
		Resource: "externalsecrets",
	}
	callback := func(o unstructured.Unstructured) {
		count = count + 1
	}

	// first pass, cluster-wide lookup
	ns := ""
	err = c.ForEachDynamicResources(context.Background(), kind, ns, callback)
	if err != nil {
		t.Errorf("unexpected error from kubernetes client foreach cluster, %v", err)
		return
	}

	if count != 3 {
		t.Errorf("unexpected count from kubernetes client foreach cluster, %d", count)
	}

	// reset, second pass, namespaced
	count = 0
	ns = "testns1"
	err = c.ForEachDynamicResources(context.Background(), kind, ns, callback)
	if err != nil {
		t.Errorf("unexpected error from kubernetes client foreach namespaced, %v", err)
		return
	}

	if count != 2 {
		t.Errorf("unexpected count from kubernetes client foreach namespaced, %d", count)
	}
}

func TestProviderKubernetesClientRefreshExternalSecrets(t *testing.T) {
	api := NewKubernetesApiMock().WithDynamicObjects(
		newK8sObject("external-secrets.io/v1beta1", "ExternalSecret", "testns1", "testobj1"),
		newK8sObject("external-secrets.io/v1beta1", "ExternalSecret", "testns2", "testobj1"),
	)

	cfg := schema.QuartzConfig{}
	kubeconfig := KubeconfigInfo{}

	c, err := NewKubernetesClient(api, kubeconfig, cfg)
	if err != nil {
		t.Errorf("unexpected error from kubernetes client constructor, %v", err)
		return
	}

	res, err := c.RefreshExternalSecrets(context.Background())
	if err != nil {
		t.Errorf("unexpected error from kubernetes client refresh secrets, %v", err)
		return
	}

	if len(res) != 2 {
		t.Errorf("unexpected response from kubernetes client refresh secrets, %v", res)
		return
	}

	for _, r := range res {
		a := r.Item.GetAnnotations()
		if !util.MapContainsKey(a, "force-sync") {
			t.Errorf("expected annotation missing, %v", a)
		}
	}
}

func TestProviderKubernetesClientGetConfigMapValue(t *testing.T) {
	cm := corev1.ConfigMap{}
	cm.Name = "testcm1"
	cm.Namespace = "testns1"
	cm.Data = map[string]string{
		"key1": "val1",
		"key2": "val2",
	}
	api := NewKubernetesApiMock().WithClientObjects(&cm)

	cfg := schema.QuartzConfig{}
	kubeconfig := KubeconfigInfo{}

	c, err := NewKubernetesClient(api, kubeconfig, cfg)
	if err != nil {
		t.Errorf("unexpected error from kubernetes client constructor, %v", err)
		return
	}

	m, err := c.GetConfigMapValue(context.Background(), "testns1", "testcm1")
	if err != nil {
		t.Errorf("unexpected error from kubernetes client get cm, %v", err)
		return
	}

	if len(m) != 2 ||
		m["key1"] != "val1" ||
		m["key2"] != "val2" {
		t.Errorf("unexpected response from kubernetes client get cm, %v", m)
	}
}

func TestProviderKubernetesClientGetSecretValue(t *testing.T) {
	secret := corev1.Secret{}
	secret.Name = "testsecret1"
	secret.Namespace = "testns1"
	secret.Data = map[string][]byte{
		"key1": []byte("val1"),
		"key2": []byte("val2"),
	}
	api := NewKubernetesApiMock().WithClientObjects(&secret)

	cfg := schema.QuartzConfig{}
	kubeconfig := KubeconfigInfo{}

	c, err := NewKubernetesClient(api, kubeconfig, cfg)
	if err != nil {
		t.Errorf("unexpected error from kubernetes client constructor, %v", err)
		return
	}

	m, err := c.GetSecretValue(context.Background(), "testns1", "testsecret1")
	if err != nil {
		t.Errorf("unexpected error from kubernetes client get secret, %v", err)
		return
	}

	if len(m) != 2 ||
		m["key1"] != "val1" ||
		m["key2"] != "val2" {
		t.Errorf("unexpected response from kubernetes client get secret, %v", m)
	}
}

func TestProviderKubernetesClientGetDynamicResource(t *testing.T) {
	api := NewKubernetesApiMock().WithDynamicObjects(
		newK8sObject("external-secrets.io/v1beta1", "ExternalSecret", "testns1", "testobj1"),
	)

	cfg := schema.QuartzConfig{}
	kubeconfig := KubeconfigInfo{}

	c, err := NewKubernetesClient(api, kubeconfig, cfg)
	if err != nil {
		t.Errorf("unexpected error from kubernetes client constructor, %v", err)
		return
	}

	kind, _ := c.LookupKind(context.Background(), "ExternalSecret")
	res, err := c.GetDynamicResource(context.Background(), kind, "testns1", "testobj1")
	if err != nil {
		t.Errorf("unexpected error from kubernetes client get dynamic, %v", err)
		return
	}

	actualName, found, err := unstructured.NestedString(res, "metadata", "name")
	if err != nil ||
		!found ||
		actualName != "testobj1" {
		t.Errorf("unexpected response from kubernetes client get dynamic, %v", res)
	}
}

func TestProviderKubernetesClientGetAppConnectionInfo(t *testing.T) {
	secret := corev1.Secret{}
	secret.Name = "test-secret"
	secret.Namespace = "test"
	secret.Data = map[string][]byte{
		"password": []byte("supersecretpassword"),
	}

	vs := newK8sObject("networking.istio.io/v1beta1", "VirtualService", "test", "test")
	unstructured.SetNestedStringSlice(vs.Object, []string{"testapp.example.com"}, "spec", "hosts")

	api := NewKubernetesApiMock().
		WithClientObjects(&secret).
		WithDynamicObjects(vs)

	opts := schema.NewApplicationLookupConfig("test", "test-secret", "test-admin", "username", "password", "")
	// opts.Ingress = schema.ApplicationLookupIngressConfig{
	// 	Kind:    "VirtualService",
	// 	Group:   "networking.istio.io",
	// 	Version: "v1beta1",
	// }

	cfg := schema.QuartzConfig{
		Core: schema.InfrastructureEnvironmentConfig{
			Applications: map[string]schema.InfrastructureApplicationConfig{
				"test": {
					Lookup: opts,
				},
			},
		},
	}
	kubeconfig := KubeconfigInfo{}

	c, err := NewKubernetesClient(api, kubeconfig, cfg)
	if err != nil {
		t.Errorf("unexpected error from kubernetes client constructor, %v", err)
		return
	}
	res := c.GetAppConnectionInfo(context.Background(), "TestApp", opts)

	if res.Error != nil ||
		res.Name != "TestApp" ||
		res.AdminUsername != "test-admin" ||
		res.AdminPassword != "supersecretpassword" ||
		res.PublicEndpoint != "testapp.example.com" {
		t.Errorf("unexpected response from kubernetes client get app info, %v", res)
	}

	c.PrintClusterInfo(context.Background())
}

func TestProviderKubernetesClientGetAppConnectionInfoEmpty(t *testing.T) {
	api := NewKubernetesApiMock()

	opts := schema.NewApplicationLookupConfig("", "", "", "", "", "")
	cfg := schema.QuartzConfig{
		Core: schema.InfrastructureEnvironmentConfig{
			Applications: map[string]schema.InfrastructureApplicationConfig{
				"test": {
					Lookup: opts,
				},
			},
		},
	}
	kubeconfig := KubeconfigInfo{}

	c, err := NewKubernetesClient(api, kubeconfig, cfg)
	if err != nil {
		t.Errorf("unexpected error from kubernetes client constructor, %v", err)
		return
	}
	res := c.GetAppConnectionInfo(context.Background(), "TestApp", opts)

	if res.Error != nil {
		t.Errorf("unexpected response from kubernetes client get app info, %v", res)
	}
}

func TestProviderKubernetesClientGetAppConnectionInfoMissingResources(t *testing.T) {
	api := NewKubernetesApiMock()

	opts := schema.NewApplicationLookupConfig("test", "test-secret", "", "username", "password", "test")
	cfg := schema.QuartzConfig{
		Core: schema.InfrastructureEnvironmentConfig{
			Applications: map[string]schema.InfrastructureApplicationConfig{
				"test": {
					Lookup: opts,
				},
			},
		},
	}
	kubeconfig := KubeconfigInfo{}

	c, err := NewKubernetesClient(api, kubeconfig, cfg)
	if err != nil {
		t.Errorf("unexpected error from kubernetes client constructor, %v", err)
		return
	}
	res := c.GetAppConnectionInfo(context.Background(), "TestApp", opts)

	if res.Error == nil {
		t.Error("expected error from kubernetes client get app info not found")
	}

	if !strings.Contains(res.Error.Error(), "secrets \"test-secret\" not found") ||
		!strings.Contains(res.Error.Error(), "virtualservices.networking.istio.io \"test\" not found") {
		t.Errorf("invalid error from kubernetes client get app info, found %v", res.Error)
	}
}

func TestProviderKubernetesClientGetAppConnectionInfoNoVirtualServiceCrd(t *testing.T) {
	api := &KubernetesApiMock{}

	opts := schema.NewApplicationLookupConfig("test", "test-secret", "", "username", "password", "")
	cfg := schema.QuartzConfig{
		Core: schema.InfrastructureEnvironmentConfig{
			Applications: map[string]schema.InfrastructureApplicationConfig{
				"test": {
					Lookup: opts,
				},
			},
		},
	}
	kubeconfig := KubeconfigInfo{}

	c, err := NewKubernetesClient(api, kubeconfig, cfg)
	if err != nil {
		t.Errorf("unexpected error from kubernetes client constructor, %v", err)
		return
	}
	res := c.GetAppConnectionInfo(context.Background(), "TestApp", opts)

	if res.Error == nil {
		t.Error("expected error from kubernetes client get app info missing crd")
		return
	}
}

func TestProviderKubernetesClientWaitConditionState(t *testing.T) {
	vs := newK8sObject("networking.istio.io/v1beta1", "VirtualService", "test", "test-vs")
	unstructured.SetNestedSlice(vs.Object, []interface{}{
		map[string]interface{}{"status": "True", "type": "THINKING..."},
	}, "status", "conditions")

	api := NewKubernetesApiMock().WithDynamicObjects(vs)

	cfg := schema.QuartzConfig{}
	kubeconfig := KubeconfigInfo{}

	c, err := NewKubernetesClient(api, kubeconfig, cfg)
	if err != nil {
		t.Errorf("unexpected error from kubernetes client constructor, %v", err)
		return
	}

	kind, err := c.LookupKind(context.Background(), "VirtualService")
	if err != nil {
		t.Errorf("failed to lookup virtualservice kind, %v", err)
		return
	}

	err = c.WaitConditionState(context.Background(), kind, "test", "test-vs", "DONE", 1)
	if err == nil {
		t.Error("this should have timed out")
	}

	err = c.WaitConditionState(context.Background(), kind, "test", "test-vs", "THINKING...", 1)
	if err != nil {
		t.Errorf("unexpected response from kubernetes client wait, %v", err)
	}
}

func TestProviderKubernetesClientGetDaemonSetStatus(t *testing.T) {
	ds := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "DaemonSet",
			"metadata": map[string]interface{}{
				"namespace": "kube-system",
				"name":      "test-daemonset",
			},
			"status": map[string]interface{}{
				"desiredNumberScheduled": int64(3),
				"numberReady":            int64(3),
			},
		},
	}

	api := NewKubernetesApiMock().
		WithDynamicObjects(ds).
		AddResources(&metav1.APIResourceList{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{Name: "daemonsets", Namespaced: true, Kind: "DaemonSet"},
			},
		})
	cfg := schema.QuartzConfig{}
	kubeconfig := KubeconfigInfo{}

	c, err := NewKubernetesClient(api, kubeconfig, cfg)
	if err != nil {
		t.Errorf("unexpected error from kubernetes client constructor, %v", err)
		return
	}

	kind, err := c.LookupKind(context.Background(), "DaemonSet")
	if err != nil {
		t.Errorf("failed to lookup daemonset kind, %v", err)
		return
	}

	ready, desired, err := c.GetDaemonSetStatus(context.Background(), kind, "kube-system", "test-daemonset")
	if err != nil {
		t.Errorf("unexpected error from GetDaemonSetStatus, %v", err)
		return
	}

	if ready != 3 {
		t.Errorf("expected ready=3, got %d", ready)
	}
	if desired != 3 {
		t.Errorf("expected desired=3, got %d", desired)
	}
}

func TestProviderKubernetesClientGetDaemonSetStatusNotReady(t *testing.T) {
	ds := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "DaemonSet",
			"metadata": map[string]interface{}{
				"namespace": "kube-system",
				"name":      "test-daemonset",
			},
			"status": map[string]interface{}{
				"desiredNumberScheduled": int64(5),
				"numberReady":            int64(2),
			},
		},
	}

	api := NewKubernetesApiMock().
		WithDynamicObjects(ds).
		AddResources(&metav1.APIResourceList{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{Name: "daemonsets", Namespaced: true, Kind: "DaemonSet"},
			},
		})
	cfg := schema.QuartzConfig{}
	kubeconfig := KubeconfigInfo{}

	c, err := NewKubernetesClient(api, kubeconfig, cfg)
	if err != nil {
		t.Errorf("unexpected error from kubernetes client constructor, %v", err)
		return
	}

	kind, err := c.LookupKind(context.Background(), "DaemonSet")
	if err != nil {
		t.Errorf("failed to lookup daemonset kind, %v", err)
		return
	}

	ready, desired, err := c.GetDaemonSetStatus(context.Background(), kind, "kube-system", "test-daemonset")
	if err != nil {
		t.Errorf("unexpected error from GetDaemonSetStatus, %v", err)
		return
	}

	if ready != 2 {
		t.Errorf("expected ready=2, got %d", ready)
	}
	if desired != 5 {
		t.Errorf("expected desired=5, got %d", desired)
	}
}

func TestProviderKubernetesClientGetDaemonSetStatusNotFound(t *testing.T) {
	api := NewKubernetesApiMock().
		AddResources(&metav1.APIResourceList{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{Name: "daemonsets", Namespaced: true, Kind: "DaemonSet"},
			},
		})
	cfg := schema.QuartzConfig{}
	kubeconfig := KubeconfigInfo{}

	c, err := NewKubernetesClient(api, kubeconfig, cfg)
	if err != nil {
		t.Errorf("unexpected error from kubernetes client constructor, %v", err)
		return
	}

	kind, err := c.LookupKind(context.Background(), "DaemonSet")
	if err != nil {
		t.Errorf("failed to lookup daemonset kind, %v", err)
		return
	}

	_, _, err = c.GetDaemonSetStatus(context.Background(), kind, "kube-system", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent daemonset")
	}
}

func TestProviderKubernetesClientCleanupStuckTerminatingPods(t *testing.T) {
	api := NewKubernetesApiMock()
	cfg := schema.QuartzConfig{}
	kubeconfig := KubeconfigInfo{}

	c, err := NewKubernetesClient(api, kubeconfig, cfg)
	if err != nil {
		t.Errorf("unexpected error from kubernetes client constructor, %v", err)
		return
	}

	// Test with no pods - should return empty list
	cleaned, err := c.CleanupStuckTerminatingPods(context.Background(), 5*60*1000000000) // 5 minutes in nanoseconds
	if err != nil {
		t.Errorf("unexpected error from CleanupStuckTerminatingPods, %v", err)
		return
	}

	if len(cleaned) != 0 {
		t.Errorf("expected 0 cleaned pods, got %d", len(cleaned))
	}
}

func newK8sObject(apiVersion, kind, namespace, name string) *unstructured.Unstructured {
	o := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"namespace": namespace,
				"name":      name,
			},
		},
	}
	return o
}

func TestProviderKubernetesClientListVirtualServices(t *testing.T) {
	vs1 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "networking.istio.io/v1beta1",
			"kind":       "VirtualService",
			"metadata": map[string]interface{}{
				"namespace": "app1",
				"name":      "my-service",
			},
			"spec": map[string]interface{}{
				"hosts":    []interface{}{"my-service.example.com"},
				"gateways": []interface{}{"istio-system/main-gateway"},
			},
		},
	}
	vs2 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "networking.istio.io/v1beta1",
			"kind":       "VirtualService",
			"metadata": map[string]interface{}{
				"namespace": "app2",
				"name":      "another-service",
			},
			"spec": map[string]interface{}{
				"hosts":    []interface{}{"another.example.com"},
				"gateways": []interface{}{"mesh"}, // mesh-only gateway
			},
		},
	}

	api := NewKubernetesApiMock().
		WithDynamicObjects(vs1, vs2).
		AddResources(&metav1.APIResourceList{
			GroupVersion: "networking.istio.io/v1beta1",
			APIResources: []metav1.APIResource{
				{Name: "virtualservices", Namespaced: true, Kind: "VirtualService"},
			},
		})
	cfg := schema.QuartzConfig{}
	kubeconfig := KubeconfigInfo{}

	c, err := NewKubernetesClient(api, kubeconfig, cfg)
	if err != nil {
		t.Errorf("unexpected error from kubernetes client constructor, %v", err)
		return
	}

	result, err := c.ListVirtualServices(context.Background())
	if err != nil {
		t.Errorf("unexpected error from ListVirtualServices, %v", err)
		return
	}

	if len(result) != 2 {
		t.Errorf("expected 2 VirtualServices, got %d", len(result))
		return
	}

	// Results should be sorted by namespace/name
	if result[0].Name != "my-service" {
		t.Errorf("expected first VirtualService to be 'my-service', got '%s'", result[0].Name)
	}
	if result[0].Namespace != "app1" {
		t.Errorf("expected first VirtualService namespace 'app1', got '%s'", result[0].Namespace)
	}
	if len(result[0].Hosts) != 1 || result[0].Hosts[0] != "my-service.example.com" {
		t.Errorf("unexpected hosts for first VirtualService: %v", result[0].Hosts)
	}
}

func TestProviderKubernetesClientListVirtualServicesNotFound(t *testing.T) {
	// Test when VirtualService CRD doesn't exist
	api := NewKubernetesApiMock()
	cfg := schema.QuartzConfig{}
	kubeconfig := KubeconfigInfo{}

	c, err := NewKubernetesClient(api, kubeconfig, cfg)
	if err != nil {
		t.Errorf("unexpected error from kubernetes client constructor, %v", err)
		return
	}

	_, err = c.ListVirtualServices(context.Background())
	if err == nil {
		t.Error("expected error when VirtualService CRD not found")
	}
}

func TestProviderKubernetesClientForEachDynamicResourcesNamespaced(t *testing.T) {
	// Test ForEachDynamicResources with a specific namespace
	ds1 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"namespace": "ns1",
				"name":      "deploy1",
			},
		},
	}
	ds2 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"namespace": "ns2",
				"name":      "deploy2",
			},
		},
	}

	api := NewKubernetesApiMock().
		WithDynamicObjects(ds1, ds2).
		AddResources(&metav1.APIResourceList{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{
				{Name: "deployments", Namespaced: true, Kind: "Deployment"},
			},
		})
	cfg := schema.QuartzConfig{}
	kubeconfig := KubeconfigInfo{}

	c, err := NewKubernetesClient(api, kubeconfig, cfg)
	if err != nil {
		t.Errorf("unexpected error from kubernetes client constructor, %v", err)
		return
	}

	kind, err := c.LookupKind(context.Background(), "Deployment")
	if err != nil {
		t.Errorf("failed to lookup Deployment kind, %v", err)
		return
	}

	var found []string
	err = c.ForEachDynamicResources(context.Background(), kind, "ns1", func(item unstructured.Unstructured) {
		found = append(found, item.GetName())
	})
	if err != nil {
		t.Errorf("unexpected error from ForEachDynamicResources, %v", err)
		return
	}

	if len(found) != 1 || found[0] != "deploy1" {
		t.Errorf("expected only deploy1 in ns1, got %v", found)
	}
}

func TestProviderKubernetesClientGetSecret(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"username": []byte("admin"),
			"password": []byte("secret123"),
		},
	}

	api := NewKubernetesApiMock().WithClientObjects(secret)
	cfg := schema.QuartzConfig{}
	kubeconfig := KubeconfigInfo{}

	c, err := NewKubernetesClient(api, kubeconfig, cfg)
	if err != nil {
		t.Errorf("unexpected error from kubernetes client constructor, %v", err)
		return
	}

	s, err := c.GetSecret(context.Background(), "default", "my-secret")
	if err != nil {
		t.Errorf("unexpected error from GetSecret, %v", err)
		return
	}

	if string(s.Data["username"]) != "admin" {
		t.Errorf("expected username 'admin', got '%s'", s.Data["username"])
	}
	if string(s.Data["password"]) != "secret123" {
		t.Errorf("expected password 'secret123', got '%s'", s.Data["password"])
	}
}

func TestProviderKubernetesClientGetSecretNotFound(t *testing.T) {
	api := NewKubernetesApiMock()
	cfg := schema.QuartzConfig{}
	kubeconfig := KubeconfigInfo{}

	c, err := NewKubernetesClient(api, kubeconfig, cfg)
	if err != nil {
		t.Errorf("unexpected error from kubernetes client constructor, %v", err)
		return
	}

	_, err = c.GetSecret(context.Background(), "default", "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent secret")
	}
}

func TestProviderKubernetesClientPrintDiscoveredVirtualServices(t *testing.T) {
	// VirtualService with external gateway (should be printed)
	vs1 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "networking.istio.io/v1beta1",
			"kind":       "VirtualService",
			"metadata": map[string]interface{}{
				"namespace": "app1",
				"name":      "my-service",
			},
			"spec": map[string]interface{}{
				"hosts":    []interface{}{"my-service.example.com"},
				"gateways": []interface{}{"istio-system/main-gateway"},
			},
		},
	}
	// VirtualService with mesh-only gateway (should be skipped)
	vs2 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "networking.istio.io/v1beta1",
			"kind":       "VirtualService",
			"metadata": map[string]interface{}{
				"namespace": "app2",
				"name":      "internal-service",
			},
			"spec": map[string]interface{}{
				"hosts":    []interface{}{"internal.svc.cluster.local"},
				"gateways": []interface{}{"mesh"},
			},
		},
	}

	api := NewKubernetesApiMock().
		WithDynamicObjects(vs1, vs2).
		AddResources(&metav1.APIResourceList{
			GroupVersion: "networking.istio.io/v1beta1",
			APIResources: []metav1.APIResource{
				{Name: "virtualservices", Namespaced: true, Kind: "VirtualService"},
			},
		})
	cfg := schema.QuartzConfig{}
	kubeconfig := KubeconfigInfo{}

	c, err := NewKubernetesClient(api, kubeconfig, cfg)
	if err != nil {
		t.Errorf("unexpected error from kubernetes client constructor, %v", err)
		return
	}

	// Test with empty exclude list - should not panic
	c.PrintDiscoveredVirtualServices(context.Background(), map[string]bool{})

	// Test with exclusion - should not panic
	c.PrintDiscoveredVirtualServices(context.Background(), map[string]bool{"my-service": true})
}

func TestProviderKubernetesClientPrintDiscoveredVirtualServicesNoCRD(t *testing.T) {
	// Test when VirtualService CRD doesn't exist - should handle gracefully
	api := NewKubernetesApiMock()
	cfg := schema.QuartzConfig{}
	kubeconfig := KubeconfigInfo{}

	c, err := NewKubernetesClient(api, kubeconfig, cfg)
	if err != nil {
		t.Errorf("unexpected error from kubernetes client constructor, %v", err)
		return
	}

	// Should not panic when CRD doesn't exist
	c.PrintDiscoveredVirtualServices(context.Background(), map[string]bool{})
}
