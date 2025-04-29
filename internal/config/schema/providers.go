package schema

// ProvidersConfig represents the configuration for various providers used in Quartz.
type ProvidersConfig struct {
	Cloud         string `koanf:"cloud"`
	Dns           string `koanf:"dns"`
	SourceControl string `koanf:"source_control"`
	Monitoring    string `koanf:"monitoring"`
	Secrets       string `koanf:"secrets"`
	Oidc          string `koanf:"oidc"`
	CiCd          string `koanf:"cicd"`
}

// NewProvidersConfig returns a new ProvidersConfig instance with default values.
func NewProvidersConfig() ProvidersConfig {
	return ProvidersConfig{
		Cloud:         "aws",
		Dns:           "aws",
		SourceControl: "github",
		Monitoring:    "cloudwatch",
		Secrets:       "aws-ssm-parameter",
		Oidc:          "keycloak",
		CiCd:          "jenkins",
	}
}
