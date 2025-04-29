package schema

// TerraformConfig represents the configuration for Terraform.
type TerraformConfig struct {
	Version string `koanf:"version"` // The version of Terraform to use.
}

// NewTerraformConfig returns a new TerraformConfig instance with default values.
func NewTerraformConfig() TerraformConfig {
	return TerraformConfig{
		Version: "1.5.7",
	}
}
