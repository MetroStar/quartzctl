package provider

import (
	"context"

	"github.com/MetroStar/quartzctl/internal/config/schema"
	"github.com/MetroStar/quartzctl/internal/util"
)

// NewImageRegistryProviderClient creates a new image registry provider client based on the configuration and secrets.
// If image repository mirroring is disabled, it initializes an Ironbank client.
// Otherwise, it initializes a GitHub client.
func NewImageRegistryProviderClient(ctx context.Context, cfg schema.QuartzConfig, secrets schema.QuartzSecrets) (Provider, error) {
	if !cfg.Mirror.ImageRepository.Enabled {
		return NewIronbankClient(util.NewHttpClientFactory(), "Ironbank", secrets.Ironbank.Username, secrets.Ironbank.Password)
	}

	return NewGithubClient(util.NewHttpClientFactory(), "Github", cfg, secrets.Github)
}
