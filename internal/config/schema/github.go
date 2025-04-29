package schema

// GithubConfig represents the configuration for GitHub integration.
type GithubConfig struct {
	TagReleaseEnabled bool           `koanf:"tag_release"`
	Webhooks          GithubWebhooks `koanf:"webhooks"`
	Organization      string         `koanf:"organization"`
}

// GithubCredentials represents the credentials for accessing GitHub.
type GithubCredentials struct {
	Username string `koanf:"username"`
	Token    string `koanf:"token"`
}

// GithubWebhooks represents the configuration for GitHub webhooks.
type GithubWebhooks struct {
	Build   bool `koanf:"build"`
	Release bool `koanf:"release"`
}

// NewGithubConfig returns a new GithubConfig instance with default values.
func NewGithubConfig() GithubConfig {
	return GithubConfig{
		Organization: "MetroStar",
		Webhooks: GithubWebhooks{
			Build:   true,
			Release: false,
		},
		TagReleaseEnabled: false,
	}
}
