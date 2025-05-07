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
	"context"

	quartzSchema "github.com/MetroStar/quartzctl/internal/config/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// KubernetesApi defines the interface for interacting with Kubernetes APIs.
type KubernetesApi interface {
	// ClientSet returns a Kubernetes clientset for interacting with core Kubernetes resources.
	ClientSet() (kubernetes.Interface, error)
	// DynamicClient returns a dynamic Kubernetes client for interacting with unstructured resources.
	DynamicClient() (dynamic.Interface, error)
	// DiscoveryClient returns a discovery client for querying API server metadata.
	DiscoveryClient() (discovery.DiscoveryInterface, error)
}

// KubernetesApiImpl is an implementation of KubernetesApi using a REST configuration.
type KubernetesApiImpl struct {
	restConfig *rest.Config // The REST configuration for Kubernetes API access.
}

// NewKubernetesApi creates a new KubernetesApi instance using the provided configuration and kubeconfig information.
// If test mode is enabled, it returns a mock implementation.
func NewKubernetesApi(ctx context.Context, cfg quartzSchema.QuartzConfig, i *KubeconfigInfo) (KubernetesApi, error) {
	r, err := newRestConfig(ctx, cfg, i)
	if err != nil {
		return KubernetesApiImpl{}, err
	}

	return KubernetesApiImpl{
		restConfig: r,
	}, nil
}

// ClientSet returns a Kubernetes clientset for interacting with core Kubernetes resources.
func (api KubernetesApiImpl) ClientSet() (kubernetes.Interface, error) {
	return kubernetes.NewForConfig(api.restConfig)
}

// DynamicClient returns a dynamic Kubernetes client for interacting with unstructured resources.
func (api KubernetesApiImpl) DynamicClient() (dynamic.Interface, error) {
	return dynamic.NewForConfig(api.restConfig)
}

// DiscoveryClient returns a discovery client for querying API server metadata.
func (api KubernetesApiImpl) DiscoveryClient() (discovery.DiscoveryInterface, error) {
	return discovery.NewDiscoveryClientForConfig(api.restConfig)
}

// newRestConfig creates a new REST configuration for Kubernetes API access.
// It uses the provided Quartz configuration and kubeconfig information.
// If service account authentication is enabled, it exchanges the cloud provider token for a Kubernetes service account token.
func newRestConfig(ctx context.Context, cfg quartzSchema.QuartzConfig, i *KubeconfigInfo) (*rest.Config, error) {
	// rest client using native credentials for a given cloud provider
	kubeconfig := i.ToKubeconfigYamlBytes(cfg)
	kc1, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	if !cfg.Auth.ServiceAccount.Enabled {
		return kc1, nil
	}

	// try to exchange the cloud provider token for a kubernetes service account
	// token with a longer lifetime
	saToken, err := requestServiceAccountToken(ctx, cfg, kc1)
	if err != nil {
		return nil, err
	}

	i.Token = saToken
	kubeconfig = i.ToKubeconfigYamlBytes(cfg)
	kc2, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	return kc2, nil
}
