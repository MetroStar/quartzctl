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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/MetroStar/quartzctl/internal/config/schema"
)

func TestOidcStageCheckId(t *testing.T) {
	sut := OidcStageCheck{App: "argocd"}
	if got := sut.Id(); got != "oidc:argocd" {
		t.Errorf("unexpected Id: %v", got)
	}
}

func TestOidcStageCheckType(t *testing.T) {
	sut := OidcStageCheck{}
	if got := sut.Type(); got != "oidc" {
		t.Errorf("unexpected Type: %v", got)
	}
}

func TestOidcStageCheckRetryOptsDefaults(t *testing.T) {
	sut := OidcStageCheck{}
	got := sut.RetryOpts()
	if got.Limit != 12 || got.WaitSeconds != 10 {
		t.Errorf("unexpected default retry opts: %+v", got)
	}
}

func TestOidcStageCheckRetryOptsCustom(t *testing.T) {
	sut := OidcStageCheck{Retry: schema.StageChecksRetryConfig{Limit: 3, WaitSeconds: 5}}
	got := sut.RetryOpts()
	if got.Limit != 3 || got.WaitSeconds != 5 {
		t.Errorf("unexpected custom retry opts: %+v", got)
	}
}

func TestOidcStageCheckRunHappy(t *testing.T) {
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "abc123", "token_type": "Bearer"})
	}))
	defer svr.Close()

	sut := OidcStageCheck{App: "argocd", ClientID: "client", ClientSecret: "secret", TokenURL: svr.URL}
	if err := sut.Run(context.TODO(), schema.QuartzConfig{}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestOidcStageCheckRunMissingAccessToken(t *testing.T) {
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"token_type": "Bearer"})
	}))
	defer svr.Close()

	sut := OidcStageCheck{App: "argocd", ClientID: "client", ClientSecret: "secret", TokenURL: svr.URL}
	if err := sut.Run(context.TODO(), schema.QuartzConfig{}); err == nil {
		t.Error("expected error for missing access_token, got nil")
	}
}

func TestOidcStageCheckRunNonOkStatus(t *testing.T) {
	svr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("invalid_client"))
	}))
	defer svr.Close()

	sut := OidcStageCheck{App: "argocd", ClientID: "client", ClientSecret: "secret", TokenURL: svr.URL}
	if err := sut.Run(context.TODO(), schema.QuartzConfig{}); err == nil {
		t.Error("expected error for non-200 status, got nil")
	}
}

func TestOidcStageCheckRunMissingCredentials(t *testing.T) {
	sut := OidcStageCheck{App: "argocd"}
	if err := sut.Run(context.TODO(), schema.QuartzConfig{}); err == nil {
		t.Error("expected error for missing client_id/client_secret, got nil")
	}
}

func TestOidcStageCheckResolveCredentialsLocal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "oidc-secret.json")
	body, _ := json.Marshal(map[string]string{"client_id": "resolved-id", "client_secret": "resolved-secret"})
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("failed to write test secret file: %v", err)
	}

	cfg := schema.QuartzConfig{
		Name:      "test-cluster",
		Providers: schema.ProvidersConfig{Secrets: "local"},
		Core:      schema.InfrastructureEnvironmentConfig{Name: "infra"},
	}

	sut := OidcStageCheck{App: "argocd", SecretPath: path}
	resolved, err := sut.resolveCredentials(context.TODO(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved["client_id"] != "resolved-id" || resolved["client_secret"] != "resolved-secret" {
		t.Errorf("unexpected resolved credentials: %+v", resolved)
	}
}

func TestOidcStageCheckResolveCredentialsUnsupportedProvider(t *testing.T) {
	cfg := schema.QuartzConfig{
		Providers: schema.ProvidersConfig{Secrets: "unsupported"},
	}

	sut := OidcStageCheck{App: "argocd", SecretPath: "/some/path"}
	if _, err := sut.resolveCredentials(context.TODO(), cfg); err == nil {
		t.Error("expected error for unsupported secret provider, got nil")
	}
}
