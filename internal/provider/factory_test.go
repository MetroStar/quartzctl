package provider

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/MetroStar/quartzctl/internal/config/schema"
)

func TestProviderFactoryLoadCloud(t *testing.T) {
	f := newTestProviderFactory()
	c, err := f.Cloud(context.Background())
	if err != nil {
		t.Errorf("unexpected error in provider factory load cloud, %v", err)
	}

	t.Logf("cloud provider -> %v", c)
}

func TestProviderFactoryLoadDns(t *testing.T) {
	f := newTestProviderFactory()

	dns, err := f.Dns(context.Background())
	if err != nil {
		t.Errorf("unexpected error in provider factory load dns, %v", err)
	}

	t.Logf("dns provider -> %v", dns)
}

func TestProviderFactoryLoadSourceControl(t *testing.T) {
	f := newTestProviderFactory()
	sc, err := f.SourceControl(context.Background())
	if err != nil {
		t.Errorf("unexpected error in provider factory load source control, %v", err)
	}

	t.Logf("source control provider -> %v", sc)
}

func TestProviderFactoryLoadImageRegistry(t *testing.T) {
	f := newTestProviderFactory()
	ir, err := f.ImageRegistry(context.Background())
	if err != nil {
		t.Errorf("unexpected error in provider factory load img reg, %v", err)
	}

	t.Logf("img reg provider -> %v", ir)
}

func TestProviderFactoryLoadKubernetes(t *testing.T) {
	f := newTestProviderFactory()
	f.cloudProviderClient = TestCloudProviderClient{
		kubeconfig: KubeconfigInfo{
			Context:              "mytestcontext",
			Cluster:              "testcluster",
			User:                 "testuser",
			Endpoint:             "http://nowhere.example.com",
			CertificateAuthority: base64.StdEncoding.EncodeToString([]byte("mytestcert")),
		},
	}

	k8s, err := f.Kubernetes(context.Background())
	if err != nil {
		t.Errorf("unexpected error in provider factory load kubernetes, %v", err)
	}

	t.Logf("kubernetes provider -> %v", k8s)
}

func newTestProviderFactory() *ProviderFactory {
	f := &ProviderFactory{
		cfg: schema.QuartzConfig{
			Name: "test",
			Providers: schema.ProvidersConfig{
				Cloud:         "local",
				SourceControl: "github",
				Dns:           "cloudflare",
			},
			Dns: schema.DnsConfig{
				Zone: "example.com",
			},
			Auth: schema.DefaultAuthConfig(),
			Mirror: schema.MirrorConfig{
				ImageRepository: schema.MirrorImageRepositoryConfig{
					Enabled: false,
				},
			},
		},
		secrets: schema.QuartzSecrets{
			Cloudflare: schema.CloudflareCredentials{
				AccountId: "test-account",
				ApiToken:  "secretapikey",
			},
			Github: schema.GithubCredentials{
				Username: "test-user-that-doesnt-exist",
				Token:    "supersecretapikey",
			},
			Ironbank: schema.IronbankCredentials{
				Username: "test-ironbank-user-that-doesnt-exist",
				Password: "mysupersecretpassword",
			},
		},
	}

	return f
}
