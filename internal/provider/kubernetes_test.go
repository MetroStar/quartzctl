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
