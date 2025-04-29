package provider

import (
	"context"
	"fmt"
	"net/http"

	"github.com/MetroStar/quartzctl/internal/util"
)

// IronbankClient represents a client for interacting with the Ironbank API.
type IronbankClient struct {
	providerName string                 // The name of the provider.
	username     string                 // The username for authentication.
	password     string                 // The password for authentication.
	httpClient   util.HttpClientFactory // The HTTP client factory for making requests.
}

// IronbankCheckAccessResult represents the result of an Ironbank access check.
type IronbankCheckAccessResult struct {
	StatusCode int    // The HTTP status code returned by the Ironbank API.
	Username   string // The username used for the access check.
	Error      error  // Any error encountered during the access check.
}

// NewIronbankClient creates a new IronbankClient instance with the specified credentials.
// Returns an error if the username or password is missing.
func NewIronbankClient(httpClient util.HttpClientFactory, providerName string, username string, password string) (*IronbankClient, error) {
	if username == "" || password == "" {
		return nil, fmt.Errorf("ironbank user/password not found")
	}

	if providerName == "" {
		providerName = "Ironbank"
	}

	return &IronbankClient{
		providerName: providerName,
		username:     username,
		password:     password,
		httpClient:   httpClient,
	}, nil
}

// ProviderName returns the name of the Ironbank provider.
func (c *IronbankClient) ProviderName() string {
	return c.providerName
}

// CheckAccess performs an access check against the Ironbank API.
// It returns an IronbankCheckAccessResult containing the result of the check.
func (c *IronbankClient) CheckAccess(ctx context.Context) ProviderCheckResult {
	ibUrl := "https://registry1.dso.mil/api/v2.0/projects/ironbank/repositories"

	client := c.httpClient.NewClient()
	req, err := http.NewRequest("GET", ibUrl, nil)
	if err != nil {
		return IronbankCheckAccessResult{
			Error:    err,
			Username: c.username,
		}
	}

	req.SetBasicAuth(c.username, c.password)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return IronbankCheckAccessResult{
			Error:    err,
			Username: c.username,
		}
	}

	defer resp.Body.Close()

	ok := 200 <= resp.StatusCode && resp.StatusCode < 300
	if !ok {
		return IronbankCheckAccessResult{
			StatusCode: resp.StatusCode,
			Username:   c.username,
			Error:      fmt.Errorf("ironbank connection status %s", resp.Status),
		}
	}

	return IronbankCheckAccessResult{
		StatusCode: resp.StatusCode,
		Username:   c.username,
		Error:      nil,
	}
}

// ToTable converts the IronbankCheckAccessResult into table headers and rows for display.
func (r IronbankCheckAccessResult) ToTable() ([]string, []ProviderCheckResultRow) {
	headers := []string{"User", "Status"}
	rows := []ProviderCheckResultRow{
		{
			Status: r.Error == nil,
			Error:  r.Error,
			Data:   []string{r.Username, fmt.Sprint(r.StatusCode)},
		},
	}

	return headers, rows
}
