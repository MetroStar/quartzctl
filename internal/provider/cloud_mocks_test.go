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

import "context"

// TestCloudProviderClient is a mock implementation of the CloudProviderClient interface for testing purposes.
type TestCloudProviderClient struct {
	kubeconfig KubeconfigInfo   // Mock kubeconfig information.
	errs       map[string]error // Mock errors for specific methods.
}

// TestCloudProviderCheckResult is a mock implementation of the ProviderCheckResult interface for testing purposes.
type TestCloudProviderCheckResult struct{}

// NewTestCloudProviderClient creates a new instance of TestCloudProviderClient with default values.
func NewTestCloudProviderClient() TestCloudProviderClient {
	return TestCloudProviderClient{
		kubeconfig: KubeconfigInfo{},
		errs:       map[string]error{},
	}
}

// ProviderName returns the name of the test cloud provider.
func (c TestCloudProviderClient) ProviderName() string {
	return "test"
}

// CheckAccess performs a mock access check for the test cloud provider.
// Always returns a TestCloudProviderCheckResult.
func (c TestCloudProviderClient) CheckAccess(context.Context) ProviderCheckResult {
	return TestCloudProviderCheckResult{}
}

// CheckConfig performs a mock configuration check for the test cloud provider.
// Returns a mock error if configured.
func (c TestCloudProviderClient) CheckConfig() error {
	return c.errs["provider__cloud__CheckConfig"]
}

// CurrentIdentity returns a mock identity for the test cloud provider.
// Returns a mock error if configured.
func (c TestCloudProviderClient) CurrentIdentity(ctx context.Context) (CloudProviderIdentity, error) {
	return CloudProviderIdentity{}, c.errs["provider__cloud__CurrentIdentity"]
}

// StateBackendInfo returns mock state backend information for the test cloud provider.
func (c TestCloudProviderClient) StateBackendInfo(stage string) CloudProviderStateBackend {
	return CloudProviderStateBackend{}
}

// CreateStateBackend performs a mock state backend creation for the test cloud provider.
// Returns a mock error if configured.
func (c TestCloudProviderClient) CreateStateBackend(ctx context.Context) error {
	return c.errs["provider__cloud__CreateStateBackend"]
}

// DestroyStateBackend performs a mock state backend destruction for the test cloud provider.
// Returns a mock error if configured.
func (c TestCloudProviderClient) DestroyStateBackend(ctx context.Context) error {
	return c.errs["provider__cloud__DestroyStateBackend"]
}

// KubeconfigInfo returns mock kubeconfig information for the test cloud provider.
// Returns a mock error if configured.
func (c TestCloudProviderClient) KubeconfigInfo(ctx context.Context) (KubeconfigInfo, error) {
	return c.kubeconfig, c.errs["provider__cloud__KubeconfigInfo"]
}

// PrintConfig is a placeholder for printing the configuration of the test cloud provider.
// TODO: Implement this method.
func (c TestCloudProviderClient) PrintConfig() {
	// TODO
}

// PrintClusterInfo is a placeholder for printing cluster information for the test cloud provider.
// Returns a mock error if configured.
func (c TestCloudProviderClient) PrintClusterInfo(ctx context.Context) error {
	// TODO
	return c.errs["provider__cloud__PrintClusterInfo"]
}

// PrepareAccount performs a mock account preparation for the test cloud provider.
// Returns a mock error if configured.
func (c TestCloudProviderClient) PrepareAccount(ctx context.Context) error {
	return c.errs["provider__cloud__PrepareAccount"]
}

// ToTable converts the TestCloudProviderCheckResult into table headers and rows for display.
// Always returns empty headers and rows.
func (r TestCloudProviderCheckResult) ToTable() ([]string, []ProviderCheckResultRow) {
	return []string{}, []ProviderCheckResultRow{}
}
