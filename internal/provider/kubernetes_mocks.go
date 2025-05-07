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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	fakeDiscoveryClient "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/dynamic"
	fakeDynamicClient "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	fakeClientSet "k8s.io/client-go/kubernetes/fake"
)

// KubernetesApiMock is a mock implementation of the IKubernetesApi interface for testing purposes.
type KubernetesApiMock struct {
	err            error                     // The error to return for API calls.
	clientObjects  []runtime.Object          // The client objects to include in the fake clientset.
	dynamicObjects []runtime.Object          // The dynamic objects to include in the fake dynamic client.
	resources      []*metav1.APIResourceList // The API resources to include in the fake discovery client.
}

// NewKubernetesApiMock creates a new instance of KubernetesApiMock with default API resources.
func NewKubernetesApiMock() *KubernetesApiMock {
	return &KubernetesApiMock{
		resources: []*metav1.APIResourceList{
			{
				GroupVersion: "external-secrets.io/v1beta1",
				APIResources: []metav1.APIResource{
					{Name: "externalsecrets", Namespaced: true, Kind: "ExternalSecret"},
				},
			},
			{
				GroupVersion: "networking.istio.io/v1beta1",
				APIResources: []metav1.APIResource{
					{Name: "virtualservices", Namespaced: true, Kind: "VirtualService"},
				},
			},
			{
				GroupVersion: "apps/v1",
				APIResources: []metav1.APIResource{
					{Name: "deployments", Namespaced: true, Kind: "Deployment"},
				},
			},
		},
	}
}

// WithClientObjects adds client objects to the mock clientset.
func (api *KubernetesApiMock) WithClientObjects(objects ...runtime.Object) *KubernetesApiMock {
	api.clientObjects = objects
	return api
}

// WithDynamicObjects adds dynamic objects to the mock dynamic client.
func (api *KubernetesApiMock) WithDynamicObjects(objects ...runtime.Object) *KubernetesApiMock {
	api.dynamicObjects = objects
	return api
}

// WithError sets the error to be returned by the mock API.
func (api *KubernetesApiMock) WithError(err error) *KubernetesApiMock {
	api.err = err
	return api
}

// AddResources adds API resources to the mock discovery client.
func (api *KubernetesApiMock) AddResources(res ...*metav1.APIResourceList) *KubernetesApiMock {
	api.resources = append(api.resources, res...)
	return api
}

// ClientSet returns a fake Kubernetes clientset populated with the mock client objects.
func (api KubernetesApiMock) ClientSet() (kubernetes.Interface, error) {
	return fakeClientSet.NewSimpleClientset(api.clientObjects...), api.err
}

// DynamicClient returns a fake dynamic client populated with the mock dynamic objects.
func (api KubernetesApiMock) DynamicClient() (dynamic.Interface, error) {
	return fakeDynamicClient.NewSimpleDynamicClient(runtime.NewScheme(), api.dynamicObjects...), api.err
}

// DiscoveryClient returns a fake discovery client populated with the mock API resources.
func (api KubernetesApiMock) DiscoveryClient() (discovery.DiscoveryInterface, error) {
	c, _ := api.ClientSet()
	d, _ := c.Discovery().(*fakeDiscoveryClient.FakeDiscovery)

	if api.resources != nil {
		d.Resources = append(d.Resources, api.resources...)
	}

	return d, api.err
}
