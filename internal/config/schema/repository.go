package schema

// RepositoryConfig represents the configuration for a repository.
type RepositoryConfig struct {
	Name         string `koanf:"repo"`
	RepoUrl      string `koanf:"repo_url"`
	Provider     string `koanf:"provider"`
	Organization string `koanf:"organization"`
	Branch       string `koanf:"branch"`
}

// ApplicationRepositoryConfig represents the configuration for an application repository.
type ApplicationRepositoryConfig struct {
	Name         string                      `koanf:"repo"`
	RepoUrl      string                      `koanf:"repo_url"`
	Provider     string                      `koanf:"provider"`
	Organization string                      `koanf:"organization"`
	Branch       string                      `koanf:"branch"`
	Type         string                      `koanf:"type"`
	Db           ApplicationDbConfig         `koanf:"db"`
	BaseUrl      string                      `koanf:"base_url"`
	CallbackUrls []ApplicationCallbackConfig `koanf:"callback_urls"`
	Keycloak     map[string]interface{}      `koanf:"keycloak"`

	// Cloud specific and other schema-less settings for the app
	Settings map[string]interface{} `koanf:"settings"`
}

// ApplicationDbConfig represents the database configuration for an application.
type ApplicationDbConfig struct {
	Enabled  bool   `koanf:"enabled"`
	Admin    bool   `koanf:"admin"`
	Username string `koanf:"username"`
	DbName   string `koanf:"db_name"`
}

// ApplicationCallbackConfig represents the callback configuration for an application.
type ApplicationCallbackConfig struct {
	Url  string `koanf:"url"`
	Path string `koanf:"path"`
}

// RepositoryConfig converts an ApplicationRepositoryConfig to a RepositoryConfig.
func (c ApplicationRepositoryConfig) RepositoryConfig() RepositoryConfig {
	return RepositoryConfig{
		Name:         c.Name,
		RepoUrl:      c.RepoUrl,
		Provider:     c.Provider,
		Organization: c.Organization,
		Branch:       c.Branch,
	}
}
