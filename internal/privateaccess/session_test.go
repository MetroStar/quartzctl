package privateaccess

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/client-go/tools/clientcmd"
)

func TestStartRejectsUnsafeAWSIdentifiersBeforeExecution(t *testing.T) {
	for _, options := range []Options{
		{Cluster: "--help", Region: "us-east-1"},
		{Cluster: "cluster,Values=other", Region: "us-east-1"},
		{Cluster: "cluster", Region: "--endpoint-url"},
		{Cluster: "cluster", Region: "us-east-1\nother"},
	} {
		if session, err := Start(context.Background(), options); session != nil || err == nil {
			t.Fatal("unsafe AWS identifiers must fail before executing AWS CLI", options)
		}
	}
}

func TestTunnelPreservesTLSAndRejectsOtherDestinations(t *testing.T) {
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.TLS.ServerName != "example.com" {
			t.Errorf("TLS SNI changed: %q", request.TLS.ServerName)
		}
		io.WriteString(w, "private-api")
	}))
	defer upstream.Close()
	address, _ := url.Parse(upstream.URL)
	tunnel, err := NewTunnel("example.com:443", address.Host)
	if err != nil {
		t.Fatal(err)
	}
	defer tunnel.Close()
	proxy, _ := url.Parse(tunnel.ProxyURL)
	transport := upstream.Client().Transport.(*http.Transport).Clone()
	transport.TLSClientConfig.InsecureSkipVerify = false
	roots := x509.NewCertPool()
	roots.AddCert(upstream.Certificate())
	transport.TLSClientConfig.RootCAs = roots
	transport.Proxy = http.ProxyURL(proxy)
	client := &http.Client{Transport: transport}
	response, err := client.Get("https://example.com/version")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatal(response.Status)
	}
	if _, err := client.Get("https://another.example/version"); err == nil {
		t.Fatal("relay must not become an unrestricted proxy")
	}
	untrusted := transport.Clone()
	untrusted.TLSClientConfig.RootCAs = x509.NewCertPool()
	if _, err := (&http.Client{Transport: untrusted}).Get("https://example.com/version"); err == nil {
		t.Fatal("SSM routing must never bypass CA verification")
	}
	response, err = http.Get(tunnel.ProxyURL + "/")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusForbidden {
		t.Fatal("non-CONNECT traffic must not be permitted")
	}
}

func TestEphemeralConfigAndEnvironmentLifecycle(t *testing.T) {
	directory := ".private-context-" + t.Name()
	os.MkdirAll(directory, 0700)
	defer os.RemoveAll(directory)
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer upstream.Close()
	address, _ := url.Parse(upstream.URL)
	tunnel, err := NewTunnel("example.com:443", address.Host)
	if err != nil {
		t.Fatal(err)
	}
	ca := base64.StdEncoding.EncodeToString(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: upstream.Certificate().Raw}))
	original := []byte("apiVersion: v1\nkind: Config\ncurrent-context: fixture\nclusters:\n- name: fixture\n  cluster:\n    server: https://example.com\n    certificate-authority-data: " + ca + "\ncontexts:\n- name: fixture\n  context:\n    cluster: fixture\n    user: fixture\nusers:\n- name: fixture\n  user:\n    token: fixture\n")
	persistent := filepath.Join(directory, "persistent.yaml")
	os.WriteFile(persistent, original, 0600)
	if err := tunnel.WriteKubeconfig(directory, original); err != nil {
		t.Fatal(err)
	}
	temporary := tunnel.KubeconfigPath
	config, err := clientcmd.LoadFromFile(temporary)
	if err != nil {
		t.Fatal(err)
	}
	cluster := config.Clusters["fixture"]
	if cluster.Server != "https://example.com" || cluster.ProxyURL != tunnel.ProxyURL || cluster.InsecureSkipTLSVerify || len(cluster.CertificateAuthorityData) == 0 {
		t.Fatal("ephemeral configuration changed TLS identity or lost explicit proxy")
	}
	info, _ := os.Stat(temporary)
	if info.Mode().Perm() != 0600 {
		t.Fatal("temporary kubeconfig is not private")
	}
	refreshed := []byte(strings.Replace(string(original), "token: fixture", "token: refreshed", 1))
	if err := tunnel.WriteKubeconfig(directory, refreshed); err != nil {
		t.Fatal(err)
	}
	updated, err := clientcmd.LoadFromFile(temporary)
	if err != nil || tunnel.KubeconfigPath != temporary || updated.AuthInfos["fixture"].Token != "refreshed" {
		t.Fatal("service-account exchange must atomically refresh authentication in the same temporary context", err)
	}
	t.Setenv("HTTP_PROXY", "http://unchanged.example:3128")
	t.Setenv(ProxyEnv, "http://127.0.0.1:1234")
	tunnel.restore = bindEnvironment(tunnel.ProxyURL, temporary)
	tunnel.Close()
	tunnel.Close()
	if err := tunnel.WriteKubeconfig(directory, original); err == nil {
		t.Fatal("cancellation must not recreate a dead ephemeral proxy context")
	}
	if _, err := os.Stat(temporary); !os.IsNotExist(err) {
		t.Fatal("dead ephemeral proxy configuration persisted after CLI exit")
	}
	content, _ := os.ReadFile(persistent)
	if string(content) != string(original) || strings.Contains(string(content), "proxy-url") {
		t.Fatal("persistent kubeconfig was mutated")
	}
	if os.Getenv(ProxyEnv) != "http://127.0.0.1:1234" || os.Getenv("HTTP_PROXY") != "http://unchanged.example:3128" {
		t.Fatal("environment was not restored or global HTTP proxy was changed")
	}
}
