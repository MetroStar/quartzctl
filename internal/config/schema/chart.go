package schema

// ChartConfig represents the configuration for a Helm chart.
type ChartConfig struct {
	Path string `koanf:"path"` // The path to the Helm chart.
}
