// Copyright 2025 Metrostar Systems, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package stages

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/MetroStar/quartzctl/internal/config/schema"
	"github.com/MetroStar/quartzctl/internal/log"
	awsConfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// OidcStageCheck represents an OIDC validation check that performs a client_credentials
// token exchange against a Keycloak realm to verify the OIDC configuration is functional.
type OidcStageCheck struct {
	// App is the application name (e.g., "argocd", "jenkins")
	App string
	// ClientID is the OIDC client ID (used if SecretPath is empty)
	ClientID string
	// ClientSecret is the OIDC client secret (used if SecretPath is empty)
	ClientSecret string
	// TokenURL is the full token endpoint URL (optional; derived from cfg if empty)
	TokenURL string
	// SecretPath is the AWS SSM parameter path to read client_id/client_secret from.
	// Supports {cluster} and {env} template variables replaced at runtime.
	// If set, overrides ClientID and ClientSecret with values from the secret.
	SecretPath string
	// Retry holds retry configuration
	Retry schema.StageChecksRetryConfig
}

// Run executes the OIDC check by performing a client_credentials token exchange.
func (c OidcStageCheck) Run(ctx context.Context, cfg schema.QuartzConfig) error {
	clientID := c.ClientID
	clientSecret := c.ClientSecret
	tokenURL := c.TokenURL

	// If SecretPath is set, resolve credentials from AWS SSM
	if c.SecretPath != "" {
		resolved, err := c.resolveCredentials(ctx, cfg)
		if err != nil {
			return fmt.Errorf("oidc check [%s]: failed to resolve credentials from %s: %w", c.App, c.SecretPath, err)
		}
		if v, ok := resolved["client_id"]; ok && clientID == "" {
			clientID = v
		}
		if v, ok := resolved["client_secret"]; ok && clientSecret == "" {
			clientSecret = v
		}
		if v, ok := resolved["token_url"]; ok && tokenURL == "" {
			tokenURL = v
		}
	}

	if clientID == "" || clientSecret == "" {
		return fmt.Errorf("oidc check [%s]: client_id and client_secret are required", c.App)
	}

	// TODO: fix for outside of AWS VPC so local DNS cache won't work when records aren't created before checking.
	resolver := &net.Resolver{
		PreferGo: true,
		Dial:     selectDNSDialer(),
	}

	tr := &http.Transport{
		// lgtm[go/disabled-certificate-check]
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // #nosec G402 - internal cluster certs
		DialContext: (&net.Dialer{
			Timeout:  10 * time.Second,
			Resolver: resolver,
		}).DialContext,
	}
	client := &http.Client{Transport: tr, Timeout: 30 * time.Second}

	if tokenURL == "" {
		tokenURL = fmt.Sprintf("https://keycloak.auth.%s/auth/realms/%s/protocol/openid-connect/token",
			cfg.Dns.Domain, cfg.Core.Name)
	}

	data := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {clientID},
		"client_secret": {clientSecret},
	}

	log.Debug("Starting OIDC validation", "app", c.App, "token_url", tokenURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("oidc check [%s]: failed to create request: %w", c.App, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		log.Debug("OIDC check connection failed", "app", c.App, "err", err)
		return fmt.Errorf("oidc check [%s]: connection failed: %w", c.App, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("oidc check [%s]: failed to read response: %w", c.App, err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Debug("OIDC check failed", "app", c.App, "status", resp.StatusCode, "body", string(body))
		return fmt.Errorf("oidc check [%s]: token endpoint returned %d: %s", c.App, resp.StatusCode, string(body))
	}

	// Verify the response contains an access_token
	var tokenResp map[string]interface{}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return fmt.Errorf("oidc check [%s]: failed to parse token response: %w", c.App, err)
	}

	if _, ok := tokenResp["access_token"]; !ok {
		return fmt.Errorf("oidc check [%s]: token response missing access_token", c.App)
	}

	log.Debug("OIDC validation successful", "app", c.App, "token_type", tokenResp["token_type"])
	return nil
}

// resolveCredentials reads OIDC credentials from AWS SSM using the configured SecretPath.
func (c OidcStageCheck) resolveCredentials(ctx context.Context, cfg schema.QuartzConfig) (map[string]string, error) {
	path := c.SecretPath
	path = strings.ReplaceAll(path, "{cluster}", cfg.Name)
	path = strings.ReplaceAll(path, "{env}", cfg.Core.Name)

	log.Debug("Resolving OIDC credentials from SSM", "path", path)

	awsCfg, err := awsConfig.LoadDefaultConfig(ctx, awsConfig.WithRegion(cfg.Aws.Region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	ssmClient := ssm.NewFromConfig(awsCfg)
	decrypt := true
	output, err := ssmClient.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           &path,
		WithDecryption: &decrypt,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get SSM parameter %s: %w", path, err)
	}

	var creds map[string]string
	if err := json.Unmarshal([]byte(*output.Parameter.Value), &creds); err != nil {
		return nil, fmt.Errorf("failed to parse SSM parameter value as JSON: %w", err)
	}

	return creds, nil
}

// Id returns the unique identifier of the OIDC stage check.
func (c OidcStageCheck) Id() string {
	return fmt.Sprintf("oidc:%s", c.App)
}

// Type returns the type of the stage check.
func (c OidcStageCheck) Type() string {
	return "oidc"
}

// RetryOpts returns the retry configuration for the OIDC stage check.
func (c OidcStageCheck) RetryOpts() schema.StageChecksRetryConfig {
	r := c.Retry.Limit
	if r <= 0 {
		r = 12 // Default: 12 retries
	}

	w := c.Retry.WaitSeconds
	if w <= 0 {
		w = 10 // Default: 10 seconds between retries
	}

	return schema.StageChecksRetryConfig{
		Limit:       r,
		WaitSeconds: w,
	}
}
