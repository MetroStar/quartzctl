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
