package provider

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/MetroStar/quartzctl/internal/config/schema"
)

func TestLocalSecretProvider(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret.json")
	if err := os.WriteFile(path, []byte(`{"client_id":"id","client_secret":"secret"}`), 0600); err != nil {
		t.Fatal(err)
	}

	cfg := schema.QuartzConfig{}
	cfg.Providers.Secrets = "local"
	secretProvider, err := NewSecretProvider(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}

	got, err := secretProvider.Resolve(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if got["client_id"] != "id" || got["client_secret"] != "secret" {
		t.Fatalf("unexpected credentials: %#v", got)
	}
}
