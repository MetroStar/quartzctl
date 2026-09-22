package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/MetroStar/quartzctl/internal/config/schema"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// SecretProvider resolves structured secret values for provider-backed checks.
type SecretProvider interface {
	Resolve(context.Context, string) (map[string]string, error)
}

// NewSecretProvider constructs the configured secret backend without leaking its
// implementation into generic stage checks.
func NewSecretProvider(ctx context.Context, cfg schema.QuartzConfig) (SecretProvider, error) {
	switch strings.ToLower(cfg.Providers.Secrets) {
	case "aws-ssm-parameter":
		return newAwsSSMSecretProvider(ctx, cfg.Aws.Region)
	case "local", "file":
		return localSecretProvider{}, nil
	default:
		return nil, fmt.Errorf("unsupported secret provider %q", cfg.Providers.Secrets)
	}
}

type localSecretProvider struct{}

func (localSecretProvider) Resolve(_ context.Context, path string) (map[string]string, error) {
	value, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read secret file %s: %w", path, err)
	}

	return parseSecretValue(value, path)
}

type awsSSMSecretProvider struct {
	client *ssm.Client
}

func newAwsSSMSecretProvider(ctx context.Context, region string) (SecretProvider, error) {
	config, err := awsConfig.LoadDefaultConfig(ctx, awsConfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}

	return awsSSMSecretProvider{client: ssm.NewFromConfig(config)}, nil
}

func (p awsSSMSecretProvider) Resolve(ctx context.Context, path string) (map[string]string, error) {
	decrypt := true
	output, err := p.client.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           &path,
		WithDecryption: &decrypt,
	})
	if err != nil {
		return nil, fmt.Errorf("get SSM parameter %s: %w", path, err)
	}
	if output.Parameter == nil || output.Parameter.Value == nil {
		return nil, fmt.Errorf("SSM parameter %s has no value", path)
	}

	return parseSecretValue([]byte(*output.Parameter.Value), path)
}

func parseSecretValue(value []byte, source string) (map[string]string, error) {
	var credentials map[string]string
	if err := json.Unmarshal(value, &credentials); err != nil {
		return nil, fmt.Errorf("parse secret value from %s as JSON: %w", source, err)
	}

	return credentials, nil
}
