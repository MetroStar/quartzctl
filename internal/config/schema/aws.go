package schema

// AwsConfig represents the configuration for AWS in Quartz.
type AwsConfig struct {
	Region string `koanf:"region"` // The AWS region to use.
}
