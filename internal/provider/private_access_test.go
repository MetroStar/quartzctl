package provider

import (
	"context"
	"encoding/base64"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"

	quartzSchema "github.com/MetroStar/quartzctl/internal/config/schema"
	"github.com/MetroStar/quartzctl/internal/privateaccess"
	"k8s.io/client-go/kubernetes"
)

func TestFactoryCopiesSharePrivateSessionCleanup(t *testing.T) {
	factory := NewProviderFactory(quartzSchema.QuartzConfig{}, quartzSchema.QuartzSecrets{})
	tunnel, err := privateaccess.NewTunnel("example.com:443", "127.0.0.1:1")
	if err != nil {
		t.Fatal(err)
	}
	defer tunnel.Close()
	factory.privateAccess.session = tunnel
	copied := *factory
	if copied.privateAccess != factory.privateAccess {
		t.Fatal("factory copy must share command-scoped session ownership")
	}
	if err := copied.ClosePrivateAccess(); err != nil {
		t.Fatal(err)
	}
	if factory.privateAccess.session != nil {
		t.Fatal("cleanup through a copied factory must clear the original session")
	}
	if err := tunnel.WriteKubeconfig(t.TempDir(), nil); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatal("shared session tunnel was not closed", err)
	}
}

func TestPrivateRESTProxyIsExplicitAndTLSVerified(t *testing.T) {
	if os.Getenv("QUARTZ_TEST_REST_PROXY_HELPER") != "1" {
		binary, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		command := exec.Command(binary, "-test.run=^TestPrivateRESTProxyIsExplicitAndTLSVerified$", "-test.v")
		command.Env = append(os.Environ(), "QUARTZ_TEST_REST_PROXY_HELPER=1")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("isolated cached-proxy regression failed: %v %s", err, output)
		}
		return
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS.ServerName != "example.com" {
			t.Error("original TLS SNI was not retained")
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/v1/namespaces/fixture/serviceaccounts/fixture/token" {
			if r.Header.Get("Authorization") != "Bearer fixture" {
				t.Error("service-account exchange did not retain original authentication")
			}
			w.Write([]byte(`{"apiVersion":"authentication.k8s.io/v1","kind":"TokenRequest","status":{"token":"fixture-exchanged","expirationTimestamp":"2099-01-01T00:00:00Z"}}`))
			return
		}
		w.Write([]byte(`{"major":"1","minor":"36","gitVersion":"v1.36.0"}`))
	}))
	defer server.Close()
	upstream, _ := url.Parse(server.URL)
	tunnel, err := privateaccess.NewTunnel("example.com:443", upstream.Host)
	if err != nil {
		t.Fatal(err)
	}
	defer tunnel.Close()
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:1")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:1")
	// Prime net/http's cached environment proxy before binding the private route.
	http.ProxyFromEnvironment(&http.Request{URL: &url.URL{Scheme: "https", Host: "example.com"}})
	t.Setenv(privateaccess.ProxyEnv, tunnel.ProxyURL)
	ca := base64.StdEncoding.EncodeToString(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw}))
	info := KubeconfigInfo{Cluster: "fixture", User: "fixture", Context: "fixture", Endpoint: "https://example.com", CertificateAuthority: ca, Token: "fixture"}
	config, err := newRestConfig(context.Background(), quartzSchema.QuartzConfig{}, &info)
	if err != nil {
		t.Fatal(err)
	}
	if config.Proxy == nil || config.Insecure {
		t.Fatal("explicit proxy or verified TLS missing")
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	version, err := client.Discovery().ServerVersion()
	if err != nil || version.GitVersion != "v1.36.0" {
		t.Fatal("Go discovery did not use the explicit private route", err)
	}
	settings := quartzSchema.QuartzConfig{}
	settings.Auth.ServiceAccount.Enabled = true
	settings.Auth.ServiceAccount.Namespace = "fixture"
	settings.Auth.ServiceAccount.Name = "fixture"
	settings.Auth.ServiceAccount.ExpirationSeconds = 3600
	exchanged, err := newRestConfig(context.Background(), settings, &info)
	if err != nil || exchanged.Proxy == nil || exchanged.BearerToken != "fixture-exchanged" {
		t.Fatal("both service-account exchange and resulting REST config require the private route", err)
	}
	exchangedClient, err := kubernetes.NewForConfig(exchanged)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exchangedClient.Discovery().ServerVersion(); err != nil {
		t.Fatal("exchanged service-account REST config lost the explicit private route", err)
	}
	config.CAData = nil
	config.CAFile = ""
	untrusted, _ := kubernetes.NewForConfig(config)
	if _, err := untrusted.Discovery().ServerVersion(); err == nil {
		t.Fatal("untrusted CA must fail")
	}
}
