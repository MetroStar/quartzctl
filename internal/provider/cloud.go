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
	"fmt"
	"strings"

	"github.com/MetroStar/quartzctl/internal/config/schema"
)

// CloudProviderClient defines the interface for cloud provider clients.
type CloudProviderClient interface {
	Provider
	// CheckConfig validates the cloud provider configuration.
	CheckConfig() error
	// CurrentIdentity retrieves the current identity of the cloud provider account.
	CurrentIdentity(ctx context.Context) (CloudProviderIdentity, error)
	// StateBackendInfo retrieves information about the state backend for the specified stage.
	StateBackendInfo(stage string) CloudProviderStateBackend
	// CreateStateBackend creates the state backend for the cloud provider.
	CreateStateBackend(ctx context.Context) error
	// DestroyStateBackend destroys the state backend for the cloud provider.
	DestroyStateBackend(ctx context.Context) error
	// KubeconfigInfo retrieves the kubeconfig information for the cloud provider.
	KubeconfigInfo(ctx context.Context) (KubeconfigInfo, error)
	// PrintConfig prints the cloud provider configuration.
	PrintConfig()
	// PrintClusterInfo prints information about the cloud provider's cluster.
	PrintClusterInfo(ctx context.Context) error
	// PrepareAccount prepares the cloud provider account for use.
	PrepareAccount(ctx context.Context) error
}

// CloudProviderIdentity represents the identity of a cloud provider account.
type CloudProviderIdentity struct {
	AccountId   string // The account ID of the cloud provider.
	AccountName string // The account name of the cloud provider.
	UserId      string // The user ID of the cloud provider account.
	UserName    string // The user name of the cloud provider account.
}

// CloudProviderStateBackend represents the state backend configuration for a cloud provider.
type CloudProviderStateBackend struct {
	Name              string   // The name of the state backend.
	InitBackendConfig []string // The initialization configuration for the state backend.
}

// CloudProviderClientOpts contains options for creating a cloud provider client.
type CloudProviderClientOpts struct {
	Provider string              // The name of the cloud provider (e.g., "aws", "local").
	Name     string              // The name of the cloud provider client.
	Region   string              // The region for the cloud provider.
	cfg      schema.QuartzConfig // The Quartz configuration.
}

// NewCloudProviderClient creates a new cloud provider client using the provided Quartz configuration.
func NewCloudProviderClient(ctx context.Context, cfg schema.QuartzConfig) (CloudProviderClient, error) {
	return NewCloudProviderClientWithOpts(ctx, CloudProviderClientOpts{
		Provider: cfg.Providers.Cloud,
		Name:     cfg.Name,
		Region:   cfg.Aws.Region,
		cfg:      cfg,
	})
}

// NewCloudProviderClientWithOpts creates a new cloud provider client using the specified options.
func NewCloudProviderClientWithOpts(ctx context.Context, o CloudProviderClientOpts) (CloudProviderClient, error) {
	provider := strings.ToLower(o.Provider)

	switch provider {
	case "aws":
		return NewLazyAwsClient(ctx, o.Name, o.Region)

	case "local":
		return LocalClient{Name: o.Name}, nil
	}

	return nil, fmt.Errorf("unsupported cloud provider")
}
