package schema

// GitopsConfig represents the configuration for GitOps in Quartz.
type GitopsConfig struct {
	Core RepositoryConfig `koanf:"core"`
	Apps RepositoryConfig `koanf:"apps"`
}

// DefaultGitopsConfig returns a new GitopsConfig instance with default values.
func DefaultGitopsConfig(p string) GitopsConfig {
	return GitopsConfig{
		Core: RepositoryConfig{
			Name:         "quartz",
			Provider:     p,
			Organization: "",
			Branch:       "main",
		},
		Apps: RepositoryConfig{
			Name:         "quartz-cicd",
			Provider:     p,
			Organization: "",
			Branch:       "", // will be updated to cluster name if not set elsewhere
		},
	}
}
