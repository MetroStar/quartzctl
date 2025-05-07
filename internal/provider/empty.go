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
)

// EmptyProvider represents a placeholder provider with no functionality.
type EmptyProvider struct {
	Name  string // The name of the provider.
	Error error  // The error associated with the provider, if any.
}

// EmptyProviderCheckResult represents the result of a check for an EmptyProvider.
type EmptyProviderCheckResult struct {
	Error error // The error associated with the check result, if any.
}

// NewEmptyProvider creates a new instance of EmptyProvider with the specified name and error.
func NewEmptyProvider(name string, err error) EmptyProvider {
	return EmptyProvider{
		Name:  name,
		Error: err,
	}
}

// ProviderName returns the name of the EmptyProvider.
func (c EmptyProvider) ProviderName() string {
	return c.Name
}

// CheckAccess performs an access check for the EmptyProvider.
// It always returns an EmptyProviderCheckResult with the associated error.
func (c EmptyProvider) CheckAccess(ctx context.Context) ProviderCheckResult {
	return EmptyProviderCheckResult{
		Error: c.Error,
	}
}

// ToTable converts the EmptyProviderCheckResult into table headers and rows for display.
func (r EmptyProviderCheckResult) ToTable() ([]string, []ProviderCheckResultRow) {
	var headers []string
	rows := []ProviderCheckResultRow{
		{Status: false, Error: r.Error},
	}

	return headers, rows
}
