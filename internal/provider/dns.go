package provider

import (
	"context"
	"strings"

	"github.com/MetroStar/quartzctl/internal/config/schema"
	"github.com/MetroStar/quartzctl/internal/util"
)

// DnsProviderClient defines the interface for DNS provider clients.
type DnsProviderClient interface {
	Provider
}

// NewDnsProviderClient creates a new DNS provider client based on the provided configuration and secrets.
// If the test mode is enabled, it returns a TestDnsProviderClient. Otherwise, it initializes the appropriate DNS provider client.
func NewDnsProviderClient(ctx context.Context, cfg schema.QuartzConfig, secrets schema.QuartzSecrets) (DnsProviderClient, error) {
	provider := strings.ToLower(cfg.Providers.Dns)

	switch provider {
	case "cloudflare":
		p, err := NewCloudflareClient(util.NewHttpClientFactory(), "Cloudflare", secrets.Cloudflare.AccountId, secrets.Cloudflare.ApiToken, cfg.Dns.Zone)
		return p, err
	}

	// not an error, just nothing to do in this case
	return nil, nil
}
