package schema

// ExportConfig represents the configuration for exporting resources in Quartz.
type ExportConfig struct {
	Path        string               `koanf:"path"`
	Annotations map[string]string    `koanf:"annotations"`
	Objects     []ExportObjectConfig `koanf:"objects"`
}

// ExportObjectConfig represents the configuration for an individual object to export.
type ExportObjectConfig struct {
	Kind      string `koanf:"kind"`
	Name      string `koanf:"name"`
	Namespace string `koanf:"namespace"`
}

// NewExportConfig returns a new ExportConfig instance with default values.
func NewExportConfig() ExportConfig {
	return ExportConfig{
		Path:        "./backup",
		Annotations: map[string]string{},
		Objects:     []ExportObjectConfig{},
	}
}
