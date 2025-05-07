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
