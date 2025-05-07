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

	"github.com/MetroStar/quartzctl/internal/log"
	"github.com/MetroStar/quartzctl/internal/util"
)

// LocalClient represents a local provider client.
type LocalClient struct {
	Name string // The name of the local cluster.
}

// ProviderName returns the name of the provider.
func (c LocalClient) ProviderName() string {
	return "Local"
}

// CheckConfig validates the configuration for the local provider.
// Always returns nil as no validation is required for the local provider.
func (c LocalClient) CheckConfig() error {
	return nil
}

// CheckAccess performs an access check for the local provider.
// Always returns an EmptyProviderCheckResult as no access check is required.
func (c LocalClient) CheckAccess(ctx context.Context) ProviderCheckResult {
	return EmptyProviderCheckResult{}
}

// CurrentIdentity returns the identity of the local provider.
// Always returns a static identity for the local provider.
func (c LocalClient) CurrentIdentity(ctx context.Context) (CloudProviderIdentity, error) {
	return CloudProviderIdentity{
		AccountId:   "local",
		AccountName: "local",
		UserId:      "local",
		UserName:    "local",
	}, nil
}

// StateBackendInfo returns the state backend information for the local provider.
// Always returns a static state backend configuration.
func (c LocalClient) StateBackendInfo(_ string) CloudProviderStateBackend {
	return CloudProviderStateBackend{
		Name:              "local",
		InitBackendConfig: []string{},
	}
}

// CreateStateBackend skips the creation of a state backend for the local provider.
// Logs a message indicating that the operation is skipped.
func (c LocalClient) CreateStateBackend(_ context.Context) error {
	log.Info("Skipping state backend creation for local provider")
	return nil
}

// DestroyStateBackend skips the destruction of a state backend for the local provider.
// Logs a message indicating that the operation is skipped.
func (c LocalClient) DestroyStateBackend(_ context.Context) error {
	log.Info("Skipping state backend destruction for local provider")
	return nil
}

// KubeconfigInfo returns an error as kubeconfig information is not supported for the local provider.
func (c LocalClient) KubeconfigInfo(ctx context.Context) (KubeconfigInfo, error) {
	return KubeconfigInfo{}, fmt.Errorf("not supported at this time")
}

// PrintConfig prints the configuration of the local provider.
// Displays the name of the local cluster in a table format.
func (c LocalClient) PrintConfig() {
	headers := []string{"Cluster"}
	rows := [][]string{{c.Name}}

	util.PrintTable(headers, rows)
}

// PrintClusterInfo performs no operation for the local provider.
// Always returns nil as no cluster information is available.
func (c LocalClient) PrintClusterInfo(ctx context.Context) error {
	return nil
}

// PrepareAccount performs no operation for the local provider.
// Always returns nil as no account preparation is required.
func (c LocalClient) PrepareAccount(ctx context.Context) error {
	return nil
}
