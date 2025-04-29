package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/MetroStar/quartzctl/internal/util"
)

// CloudflareClient represents a client for interacting with the Cloudflare API.
type CloudflareClient struct {
	providerName string                 // The name of the provider.
	accountId    string                 // The Cloudflare account ID.
	apiToken     string                 // The API token for authentication.
	domain       string                 // The domain to manage.
	httpClient   util.HttpClientFactory // The HTTP client factory for making requests.
}

// CloudflareAccessCheckResult represents the result of a Cloudflare access check.
type CloudflareAccessCheckResult struct {
	Status   bool                    // Indicates whether the access check was successful.
	Error    error                   // Contains any error encountered during the check.
	Response CloudflareZonesResponse // The response from the Cloudflare API.
}

// CloudflareZonesResponse represents the response from the Cloudflare API for zones.
type CloudflareZonesResponse struct {
	Success  bool                            // Indicates whether the API call was successful.
	Errors   []string                        // Contains any errors returned by the API.
	Messages []string                        // Contains any messages returned by the API.
	Result   []CloudflareZonesResponseResult // The list of zones returned by the API.
}

// CloudflareZonesResponseResult represents a single zone in the Cloudflare API response.
type CloudflareZonesResponseResult struct {
	Id          string   // The ID of the zone.
	Name        string   // The name of the zone.
	Permissions []string // The permissions associated with the zone.
}

// NewCloudflareClient creates a new CloudflareClient instance.
// Returns an error if required parameters (accountId, apiToken, or domain) are missing.
func NewCloudflareClient(httpClient util.HttpClientFactory, providerName string, accountId string, apiToken string, domain string) (CloudflareClient, error) {
	if accountId == "" || apiToken == "" || domain == "" {
		return CloudflareClient{}, fmt.Errorf("cloudflare credentials/domain not found")
	}

	if providerName == "" {
		providerName = "Cloudflare"
	}

	return CloudflareClient{
		providerName: providerName,
		accountId:    accountId,
		apiToken:     apiToken,
		domain:       domain,
		httpClient:   httpClient,
	}, nil
}

// ProviderName returns the name of the Cloudflare provider.
func (c CloudflareClient) ProviderName() string {
	return c.providerName
}

// CheckAccess checks access to the Cloudflare API for the specified domain and account.
// It verifies the required permissions and returns the result as a CloudflareAccessCheckResult.
func (c CloudflareClient) CheckAccess(ctx context.Context) ProviderCheckResult {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones?name=%s&status=active&account.id=%s", c.domain, c.accountId)

	client := c.httpClient.NewClient()
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return CloudflareAccessCheckResult{
			Status: false,
			Error:  err,
		}
	}

	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.apiToken))

	resp, err := client.Do(req)
	if err != nil {
		return CloudflareAccessCheckResult{
			Status: false,
			Error:  err,
		}
	}

	defer resp.Body.Close()

	ok := 200 <= resp.StatusCode && resp.StatusCode < 300
	if !ok {
		return CloudflareAccessCheckResult{
			Status: false,
			Error:  fmt.Errorf("cloudflare connection status %s", resp.Status),
		}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return CloudflareAccessCheckResult{
			Status: false,
			Error:  err,
		}
	}

	var zones CloudflareZonesResponse
	err = json.Unmarshal(body, &zones)
	if err != nil {
		return CloudflareAccessCheckResult{
			Status: false,
			Error:  err,
		}
	}

	requiredPermissions := []string{"#zone:read", "#dns_records:edit", "#dns_records:read"}
	for _, p := range requiredPermissions {
		if !slices.Contains(zones.Result[0].Permissions, p) {
			err = fmt.Errorf("insufficient permissions")
			break
		}
	}

	return CloudflareAccessCheckResult{
		Status:   true,
		Error:    err,
		Response: zones,
	}
}

// ToTable converts the CloudflareAccessCheckResult into table headers and rows for display.
func (r CloudflareAccessCheckResult) ToTable() ([]string, []ProviderCheckResultRow) {
	zones := r.Response

	var err error
	if r.Error != nil {
		err = r.Error
	} else if len(zones.Errors) > 0 {
		err = errors.New(strings.Join(zones.Errors, ", "))
	}

	if len(zones.Result) == 0 {
		return nil, []ProviderCheckResultRow{
			{Status: r.Status, Error: err},
		}
	}

	headers := []string{"Zone", "ID", "Permissions", "Messages"}
	var rows []ProviderCheckResultRow
	for _, v := range zones.Result {
		rows = append(rows, ProviderCheckResultRow{
			Status: r.Status,
			Error:  err,
			Data: []string{
				v.Name,
				v.Id,
				strconv.FormatBool(r.Status),
				strings.Join(zones.Messages, ", "),
			},
		})
	}

	return headers, rows
}
