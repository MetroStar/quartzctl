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

	"github.com/MetroStar/quartzctl/internal/config/schema"
	"github.com/MetroStar/quartzctl/internal/util"
)

// ProviderFactory is responsible for creating and managing provider clients.
type ProviderFactory struct {
	cfg     schema.QuartzConfig  // The Quartz configuration.
	secrets schema.QuartzSecrets // The Quartz secrets.

	cloudProviderClient CloudProviderClient      // The cloud provider client.
	dnsProviderClient   DnsProviderClient        // The DNS provider client.
	scProviderClient    Provider                 // The source control provider client.
	imgProviderClient   Provider                 // The image registry provider client.
	k8sClient           KubernetesProviderClient // The Kubernetes provider client.
}

// Provider defines the interface for all providers.
type Provider interface {
	// ProviderName returns the name of the provider.
	ProviderName() string
	// CheckAccess performs an access check for the provider.
	CheckAccess(context.Context) ProviderCheckResult
}

type ProviderFactoryOption func(*ProviderFactory)

// NewProviderFactory creates a new ProviderFactory with the given configuration and secrets.
func NewProviderFactory(cfg schema.QuartzConfig, secrets schema.QuartzSecrets, opts ...ProviderFactoryOption) *ProviderFactory {
	factory := &ProviderFactory{
		cfg:     cfg,
		secrets: secrets,
	}

	for _, opt := range opts {
		opt(factory)
	}

	return factory
}

// NewSourceControlProviderClient creates a new source control provider client.
func NewSourceControlProviderClient(ctx context.Context, cfg schema.QuartzConfig, secrets schema.QuartzSecrets) (Provider, error) {
	p, err := NewGithubClient(util.NewHttpClientFactory(), "Github", cfg, secrets.Github)
	return p, err
}

// Kubernetes returns the Kubernetes provider client, initializing it if necessary.
func (f *ProviderFactory) Kubernetes(ctx context.Context) (KubernetesProviderClient, error) {
	if f.k8sClient != nil {
		return f.k8sClient, nil
	}

	cp, err := f.Cloud(ctx)
	if err != nil {
		return nil, err
	}

	i, err := cp.KubeconfigInfo(ctx)
	if err != nil {
		return nil, err
	}

	api, err := NewKubernetesApi(ctx, f.cfg, &i)
	if err != nil {
		return KubernetesClient{}, err
	}

	c, err := NewKubernetesClient(api, i, f.cfg)
	if err != nil {
		return nil, err
	}

	f.k8sClient = c
	return f.k8sClient, nil
}

// Cloud returns the cloud provider client, initializing it if necessary.
func (f *ProviderFactory) Cloud(ctx context.Context) (CloudProviderClient, error) {
	if f.cloudProviderClient != nil {
		return f.cloudProviderClient, nil
	}

	c, err := NewCloudProviderClient(ctx, f.cfg)
	if err != nil {
		return nil, err
	}

	f.cloudProviderClient = c
	return f.cloudProviderClient, nil
}

// Dns returns the DNS provider client, initializing it if necessary.
func (f *ProviderFactory) Dns(ctx context.Context) (DnsProviderClient, error) {
	if f.dnsProviderClient != nil {
		return f.dnsProviderClient, nil
	}

	c, err := NewDnsProviderClient(ctx, f.cfg, f.secrets)
	if err != nil {
		return nil, err
	}

	f.dnsProviderClient = c
	return f.dnsProviderClient, nil
}

// SourceControl returns the source control provider client, initializing it if necessary.
func (f *ProviderFactory) SourceControl(ctx context.Context) (Provider, error) {
	if f.scProviderClient != nil {
		return f.scProviderClient, nil
	}

	c, err := NewSourceControlProviderClient(ctx, f.cfg, f.secrets)
	if err != nil {
		return nil, err
	}

	f.scProviderClient = c
	return f.scProviderClient, nil
}

// ImageRegistry returns the image registry provider client, initializing it if necessary.
func (f *ProviderFactory) ImageRegistry(ctx context.Context) (Provider, error) {
	if f.imgProviderClient != nil {
		return f.imgProviderClient, nil
	}

	c, err := NewImageRegistryProviderClient(ctx, f.cfg, f.secrets)
	if err != nil {
		return nil, err
	}

	f.imgProviderClient = c
	return f.imgProviderClient, nil
}

// WithConfig sets the Quartz configuration and returns the updated factory.
func WithConfig(c schema.QuartzConfig) ProviderFactoryOption {
	return func(f *ProviderFactory) {
		f.cfg = c
	}
}

// WithSecrets sets the Quartz secrets and returns the updated factory.
func WithSecrets(s schema.QuartzSecrets) ProviderFactoryOption {
	return func(f *ProviderFactory) {
		f.secrets = s
	}
}

// WithCloudProvider sets the cloud provider client and returns the updated factory.
func WithCloudProvider(p CloudProviderClient) ProviderFactoryOption {
	return func(f *ProviderFactory) {
		f.cloudProviderClient = p
	}
}

// WithDnsProvider sets the DNS provider client and returns the updated factory.
func WithDnsProvider(p DnsProviderClient) ProviderFactoryOption {
	return func(f *ProviderFactory) {
		f.dnsProviderClient = p
	}
}

// WithSourceControlProvider sets the source control provider client and returns the updated factory.
func WithSourceControlProvider(p Provider) ProviderFactoryOption {
	return func(f *ProviderFactory) {
		f.scProviderClient = p
	}
}

// WithImageRegistryProvider sets the image registry provider client and returns the updated factory.
func WithImageRegistryProvider(p Provider) ProviderFactoryOption {
	return func(f *ProviderFactory) {
		f.imgProviderClient = p
	}
}

// WithKubernetesProvider sets the Kubernetes provider client and returns the updated factory.
func WithKubernetesProvider(p KubernetesProviderClient) ProviderFactoryOption {
	return func(f *ProviderFactory) {
		f.k8sClient = p
	}
}
