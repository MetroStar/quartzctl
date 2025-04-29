package schema

// InternalConfig exposes options to customize features not yet or ever intended for common usage.
type InternalConfig struct {
	Installer InstallerConfig `koanf:"installer"`
}

// InstallerConfig represents the configuration for the installer.
type InstallerConfig struct {
	Summary InstallerSummaryConfig `koanf:"summary"`
}

// InstallerSummaryConfig represents the configuration for the installer summary.
type InstallerSummaryConfig struct {
	Enabled bool `koanf:"enabled"`
}

// NewInternalConfig returns a new InternalConfig instance with default values.
func NewInternalConfig() InternalConfig {
	return InternalConfig{
		Installer: InstallerConfig{
			Summary: InstallerSummaryConfig{
				Enabled: true,
			},
		},
	}
}
