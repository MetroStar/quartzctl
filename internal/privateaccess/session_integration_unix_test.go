//go:build !windows

package privateaccess

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// The fake AWS executable models SSM's TCP forwarding, not AWS/TLS authorization.
func TestSessionManagerHelper(t *testing.T) {
	if os.Getenv("QUARTZ_TEST_SSM_HELPER") != "1" {
		return
	}
	start := 0
	for index, argument := range os.Args {
		if argument == "--" {
			start = index + 1
			break
		}
	}
	arguments := os.Args[start:]
	if len(arguments) < 2 {
		os.Exit(2)
	}
	switch arguments[0] + " " + arguments[1] {
	case "ec2 describe-instances":
		fmt.Print(`["i-00000000000000001"]`)
		os.Exit(0)
	case "ssm describe-instance-information":
		fmt.Print(`["Online"]`)
		os.Exit(0)
	case "ssm terminate-session":
		foundID := false
		for _, argument := range arguments {
			if argument == "--session-id=fixture-session" {
				foundID = true
			}
		}
		if !foundID {
			os.Exit(8)
		}
		if os.Getenv("QUARTZ_TEST_SSM_TERMINATE_FAIL") == "1" {
			os.Exit(7)
		}
		os.WriteFile(os.Getenv("QUARTZ_TEST_SSM_TERMINATED"), []byte("terminated"), 0600)
		os.Exit(0)
	case "ssm start-session":
		fmt.Print("Starting session with SessionId: fixture-session\n")
		var parameters map[string][]string
		for index, argument := range arguments {
			if argument == "--parameters" {
				json.Unmarshal([]byte(arguments[index+1]), &parameters)
			}
		}
		if len(parameters["host"]) != 1 || parameters["host"][0] != "example.com" || parameters["portNumber"][0] != "443" {
			os.Exit(3)
		}
		listener, err := net.Listen("tcp", "127.0.0.1:"+parameters["localPortNumber"][0])
		if err != nil {
			os.Exit(4)
		}
		for {
			connection, err := listener.Accept()
			if err != nil {
				os.Exit(0)
			}
			go func() {
				defer connection.Close()
				upstream, err := net.Dial("tcp", os.Getenv("QUARTZ_TEST_SSM_UPSTREAM"))
				if err != nil {
					return
				}
				defer upstream.Close()
				go io.Copy(upstream, connection)
				io.Copy(connection, upstream)
			}()
		}
	default:
		os.Exit(5)
	}
}

func TestMockedSSMStartPrivateKubeconfigAndClean(t *testing.T) {
	directory := ".ssm-e2e-" + t.Name()
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(directory)
	absolute, _ := filepath.Abs(directory)
	binary, _ := os.Executable()
	shim := []byte("#!/bin/sh\nexec \"$QUARTZ_TEST_BINARY\" -test.run=TestSessionManagerHelper -- \"$@\"\n")
	for _, name := range []string{"aws", "session-manager-plugin"} {
		if err := os.WriteFile(filepath.Join(directory, name), shim, 0700); err != nil {
			t.Fatal(err)
		}
	}
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.TLS.ServerName != "example.com" {
			t.Error("private SSM forwarding changed SNI")
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/version":
			io.WriteString(w, `{"major":"1","minor":"36","gitVersion":"v1.36.0"}`)
		case strings.HasSuffix(r.URL.Path, "/secrets"):
			io.WriteString(w, `{"apiVersion":"v1","kind":"SecretList","metadata":{},"items":[]}`)
		case r.URL.Path == "/api":
			io.WriteString(w, `{"apiVersion":"v1","kind":"APIVersions","versions":["v1"],"serverAddressByClientCIDRs":[]}`)
		case r.URL.Path == "/apis":
			io.WriteString(w, `{"apiVersion":"v1","kind":"APIGroupList","groups":[]}`)
		case r.URL.Path == "/api/v1":
			io.WriteString(w, `{"apiVersion":"v1","kind":"APIResourceList","groupVersion":"v1","resources":[{"name":"secrets","singularName":"","namespaced":true,"kind":"Secret","verbs":["get","list"]}]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	target, _ := url.Parse(upstream.URL)
	t.Setenv("PATH", absolute+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("QUARTZ_TEST_SSM_HELPER", "1")
	t.Setenv("QUARTZ_TEST_BINARY", binary)
	t.Setenv("QUARTZ_TEST_SSM_UPSTREAM", target.Host)
	terminated := filepath.Join(absolute, "terminated")
	t.Setenv("QUARTZ_TEST_SSM_TERMINATED", terminated)
	t.Setenv(ProxyEnv, "")
	t.Setenv(ConfigEnv, "")
	ca := base64.StdEncoding.EncodeToString(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: upstream.Certificate().Raw}))
	kubeconfig := []byte("apiVersion: v1\nkind: Config\ncurrent-context: fixture\nclusters:\n- name: fixture\n  cluster:\n    server: https://example.com\n    certificate-authority-data: " + ca + "\ncontexts:\n- name: fixture\n  context:\n    cluster: fixture\n    user: fixture\nusers:\n- name: fixture\n  user:\n    token: fixture\n")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	session, err := Start(ctx, Options{Cluster: "fixture", Region: "us-east-1", Endpoint: "https://example.com", Directory: absolute, Kubeconfig: kubeconfig})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	config, err := clientcmd.BuildConfigFromFlags("", session.KubeconfigPath)
	if err != nil {
		t.Fatal(err)
	}
	transport, err := rest.TransportFor(config)
	if err != nil {
		t.Fatal(err)
	}
	response, err := (&http.Client{Transport: transport}).Get("https://example.com/version")
	if err != nil {
		t.Fatal(err)
	}
	content, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if !strings.Contains(string(content), `"gitVersion":"v1.36.0"`) {
		t.Fatal("temporary kubeconfig failed to route through mocked SSM")
	}
	for _, command := range [][]string{
		{"kubectl", "--kubeconfig", session.KubeconfigPath, "get", "--raw=/version"},
		{"helm", "list", "--kubeconfig", session.KubeconfigPath, "--namespace", "fixture", "--output", "json"},
	} {
		path, err := exec.LookPath(command[0])
		if err != nil {
			t.Logf("%s missing: native client integration not run", command[0])
			continue
		}
		output, err := exec.CommandContext(ctx, path, command[1:]...).CombinedOutput()
		if err != nil {
			t.Fatalf("native %s did not route through the same private context: %v %s", command[0], err, output)
		}
	}
	if os.Getenv(ProxyEnv) != session.ProxyURL || os.Getenv(ConfigEnv) != session.KubeconfigPath {
		t.Fatal("Go, providers and kubectl did not receive one consistent temporary context")
	}
	testNativePrivateProviders(t, ctx, absolute, session)
	cancel()
	deadline := time.Now().Add(3 * time.Second)
	for {
		_, err := os.Stat(session.KubeconfigPath)
		if os.IsNotExist(err) && os.Getenv(ProxyEnv) == "" && os.Getenv(ConfigEnv) == "" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("canceled install/acceptance/clean did not remove the proxy context")
		}
		time.Sleep(10 * time.Millisecond)
	}
	select {
	case <-session.wait:
	case <-time.After(3 * time.Second):
		t.Fatal("SSM forwarding helper was left running")
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	if content, err := os.ReadFile(terminated); err != nil || string(content) != "terminated" {
		t.Fatal("canceled lifecycle did not explicitly terminate the remote session", err)
	}
}
