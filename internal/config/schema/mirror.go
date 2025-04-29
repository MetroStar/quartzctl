package schema

// MirrorConfig represents the configuration for mirroring resources in Quartz.
type MirrorConfig struct {
	ImageRepository MirrorImageRepositoryConfig `koanf:"image_repository"`
	Grype           bool                        `koanf:"grype"`
}

// MirrorImageRepositoryConfig represents the configuration for mirroring image repositories.
type MirrorImageRepositoryConfig struct {
	Enabled          bool     `koanf:"enabled"`
	Target           string   `koanf:"target"`
	SourceRegistries []string `koanf:"source_registries"`
}

// NewMirrorConfig returns a new MirrorConfig instance with default values.
func NewMirrorConfig() MirrorConfig {
	return MirrorConfig{
		ImageRepository: MirrorImageRepositoryConfig{
			Enabled: true,
			Target:  "ghcr.io/metrostar/quartz-pkgs",
			SourceRegistries: []string{
				"registry1.dso.mil",
				"registry.dso.mil",
				"quay.io",
			},
		},
	}
}
