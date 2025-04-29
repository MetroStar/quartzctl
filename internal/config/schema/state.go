package schema

// StateConfig represents the configuration for state management in Quartz.
type StateConfig struct {
	Enabled            bool   `koanf:"enabled"`            // Indicates if state management is enabled.
	ConfigMapName      string `koanf:"configMapName"`      // The name of the ConfigMap used for state management.
	ConfigMapNamespace string `koanf:"configMapNamespace"` // The namespace of the ConfigMap used for state management.
}

// NewStateConfig returns a new StateConfig instance with default values.
func NewStateConfig() StateConfig {
	return StateConfig{
		Enabled:            true,
		ConfigMapName:      "quartz-install-state",
		ConfigMapNamespace: "quartz",
	}
}
